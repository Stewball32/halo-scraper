// Command memscan scans Xbox RAM in a running xemu instance for known
// UTF-16LE strings or dumps specific memory regions.
//
// Scan mode: sudo go run ./cmd/memscan --socket <path>
// Dump mode: sudo go run ./cmd/memscan --socket <path> --dump 0x83606A00:512,0x83691880:816
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"xemu-cartographer/internal/xemu"
)

func main() {
	socket := flag.String("socket", "", "QMP socket path")
	dump := flag.String("dump", "", "Dump specific regions: GVA:size,GVA:size,...")
	dumpAll := flag.String("dump-all", "", "Dump full Xbox RAM (55MB) to file")
	scanSig := flag.Bool("scan-signature", false, "Scan game state region for d@t@ (0x64407440) datum array headers")
	scanPtr := flag.String("scan-u32", "", "Scan all RAM for a uint32 value (hex, e.g. 0x82D499F0)")
	scanStr := flag.String("scan-utf16", "", "Scan all RAM for a UTF-16LE string (e.g. an xbox console name)")
	flag.Parse()

	if *socket == "" {
		log.Fatal("--socket is required")
	}

	inst := &xemu.Instance{Name: "memscan", QMPSock: *socket}
	if err := inst.Init(nil); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer inst.Close()

	if *dumpAll != "" {
		dumpFullRAM(inst, *dumpAll)
		return
	}

	if *scanSig {
		scanSignatures(inst)
		return
	}

	if *scanPtr != "" {
		scanU32(inst, *scanPtr)
		return
	}

	if *scanStr != "" {
		scanUTF16(inst, *scanStr)
		return
	}

	if *dump != "" {
		dumpRegions(inst, *dump)
		return
	}

	scanMode(inst)
}

func dumpRegions(inst *xemu.Instance, spec string) {
	for _, part := range strings.Split(spec, ",") {
		parts := strings.SplitN(part, ":", 2)
		if len(parts) != 2 {
			log.Printf("invalid dump spec %q (expected GVA:size)", part)
			continue
		}
		gva, err := strconv.ParseUint(strings.TrimPrefix(parts[0], "0x"), 16, 32)
		if err != nil {
			log.Printf("invalid GVA %q: %v", parts[0], err)
			continue
		}
		size, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Printf("invalid size %q: %v", parts[1], err)
			continue
		}

		data, err := inst.Mem.ReadBytes(uint32(gva), size)
		if err != nil {
			log.Printf("read error at GVA 0x%08X: %v", gva, err)
			continue
		}

		fmt.Printf("\n=== DUMP GVA 0x%08X (%d bytes) ===\n", gva, size)
		hexDump(data, uint32(gva))
	}
}

func scanU32(inst *xemu.Instance, valStr string) {
	mem := inst.Mem

	val, err := strconv.ParseUint(strings.TrimPrefix(valStr, "0x"), 16, 32)
	if err != nil {
		log.Fatalf("invalid u32 value %q: %v", valStr, err)
	}

	needle := make([]byte, 4)
	binary.LittleEndian.PutUint32(needle, uint32(val))

	const startGVA = uint32(0x80000000)
	const endGVA = uint32(0x83700000)
	const chunkSize = 1 << 20

	log.Printf("scanning GVA 0x%08X–0x%08X for u32 0x%08X...", startGVA, endGVA, uint32(val))

	for gva := startGVA; gva < endGVA; gva += chunkSize {
		readSize := chunkSize
		if gva+uint32(readSize) > endGVA {
			readSize = int(endGVA - gva)
		}
		data, err := mem.ReadBytes(gva, readSize)
		if err != nil {
			continue
		}

		// Skip unmapped chunks (all 0xFF).
		allFF := true
		for i := 0; i < 256 && i < len(data); i++ {
			if data[i] != 0xFF {
				allFF = false
				break
			}
		}
		if allFF {
			continue
		}

		for i := 0; i <= len(data)-4; i += 4 {
			if bytes.Equal(data[i:i+4], needle) {
				hitGVA := gva + uint32(i)
				fmt.Printf("FOUND 0x%08X at GVA 0x%08X\n", uint32(val), hitGVA)
			}
		}
	}

	log.Println("scan complete")
}

func scanUTF16(inst *xemu.Instance, needleStr string) {
	mem := inst.Mem
	needle := utf16le(needleStr)

	const startGVA = uint32(0x80000000)
	const endGVA = uint32(0x83700000)
	const chunkSize = 1 << 20

	log.Printf("scanning GVA 0x%08X–0x%08X for UTF-16LE %q...", startGVA, endGVA, needleStr)

	hits := 0
	for gva := startGVA; gva < endGVA; gva += chunkSize {
		readSize := chunkSize
		if gva+uint32(readSize) > endGVA {
			readSize = int(endGVA - gva)
		}
		data, err := mem.ReadBytes(gva, readSize)
		if err != nil {
			continue
		}

		allFF := true
		for i := 0; i < 256 && i < len(data); i++ {
			if data[i] != 0xFF {
				allFF = false
				break
			}
		}
		if allFF {
			continue
		}

		for i := 0; i <= len(data)-len(needle); i++ {
			if bytes.Equal(data[i:i+len(needle)], needle) {
				hitGVA := gva + uint32(i)
				start := max(0, i-32)
				end := min(len(data), i+len(needle)+96)
				fmt.Printf("\n=== FOUND %q at GVA 0x%08X (phys 0x%06X) ===\n",
					needleStr, hitGVA, hitGVA-0x80000000)
				hexDump(data[start:end], gva+uint32(start))
				hits++
			}
		}
	}

	log.Printf("scan complete — %d hit(s)", hits)
}

func scanSignatures(inst *xemu.Instance) {
	mem := inst.Mem

	// Scan the game state region (4 MB from 0x80061000) plus extended range
	// up through where we know session data lives (0x836xxxxx).
	// The d@t@ signature 0x64407440 appears at +0x28 in every s_data_array header.
	const signature uint32 = 0x64407440

	// Scan in 1MB chunks from 0x80061000 to 0x83700000 to cover the full range.
	const startGVA = uint32(0x80061000)
	const endGVA = uint32(0x83700000)
	const chunkSize = 1 << 20

	log.Printf("scanning GVA 0x%08X–0x%08X for d@t@ signature (0x%08X)...", startGVA, endGVA, signature)

	sigBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sigBytes, signature)

	for gva := startGVA; gva < endGVA; gva += chunkSize {
		readSize := chunkSize
		if gva+uint32(readSize) > endGVA {
			readSize = int(endGVA - gva)
		}
		data, err := mem.ReadBytes(gva, readSize)
		if err != nil {
			continue
		}

		// Check if chunk is unmapped (all 0xFF).
		allFF := true
		for i := 0; i < 256 && i < len(data); i++ {
			if data[i] != 0xFF {
				allFF = false
				break
			}
		}
		if allFF {
			continue
		}

		// Search for signature bytes.
		for i := 0; i <= len(data)-4; i++ {
			if bytes.Equal(data[i:i+4], sigBytes) {
				hitGVA := gva + uint32(i)
				// Halo 2's s_data_array has an extra 4-byte field before the
				// signature compared to CE, so signature is at +0x2C not +0x28.
				headerGVA := hitGVA - 0x2C

				// Read the full header (0x4C bytes).
				header, err := mem.ReadBytes(headerGVA, 0x4C)
				if err != nil || len(header) < 0x4C {
					fmt.Printf("\n=== d@t@ HIT at GVA 0x%08X (header 0x%08X) — read error ===\n", hitGVA, headerGVA)
					continue
				}

				// Parse header fields.
				name := ""
				for j := 0; j < 32; j++ {
					if header[j] == 0 {
						name = string(header[:j])
						break
					}
				}
				if name == "" {
					name = "(empty)"
				}

				maxCount := binary.LittleEndian.Uint16(header[0x20:])
				datumSize := binary.LittleEndian.Uint16(header[0x22:])
				activeCount := int32(binary.LittleEndian.Uint32(header[0x3C:]))
				dataPtr := binary.LittleEndian.Uint32(header[0x48:])

				fmt.Printf("\n=== DATUM ARRAY: %-24s ===\n", name)
				fmt.Printf("  Header GVA:   0x%08X\n", headerGVA)
				fmt.Printf("  Max count:    %d\n", maxCount)
				fmt.Printf("  Datum size:   0x%04X (%d bytes)\n", datumSize, datumSize)
				fmt.Printf("  Active count: %d\n", activeCount)
				fmt.Printf("  Data pointer: 0x%08X\n", dataPtr)

				// Dump the first datum if there's active data.
				if activeCount > 0 && dataPtr >= 0x80000000 && datumSize > 0 && datumSize < 0x1000 {
					dumpSize := int(datumSize)
					if dumpSize > 256 {
						dumpSize = 256
					}
					firstDatum, err := mem.ReadBytes(dataPtr, dumpSize)
					if err == nil {
						fmt.Printf("  First datum (%d bytes):\n", dumpSize)
						hexDump(firstDatum, dataPtr)
					}
				}
			}
		}
	}

	log.Println("signature scan complete")
}

func scanMode(inst *xemu.Instance) {
	// UTF-16LE patterns for known gamertags.
	patterns := map[string][]byte{
		"uno":  utf16le("uno"),
		"dos":  utf16le("dos"),
		"tres": utf16le("tres"),
	}

	// Also scan for ASCII map path fragments.
	asciiPatterns := map[string][]byte{
		"scenarios\\": []byte("scenarios\\"),
		"midship":     []byte("midship"),
		"lockout":     []byte("lockout"),
		"coagulation": []byte("coagulation"),
		"zanzibar":    []byte("zanzibar"),
		"ascension":   []byte("ascension"),
	}

	const chunkSize = 1 << 20 // 1 MB
	const totalSize = 60 << 20 // 60 MB (skip last 4 MB GPU region)

	log.Printf("scanning %d MB of Xbox RAM...", totalSize>>20)

	for offset := uint32(0); offset < uint32(totalSize); offset += chunkSize {
		gva := uint32(0x80000000) + offset
		data, err := inst.Mem.ReadBytes(gva, chunkSize)
		if err != nil {
			log.Printf("read error at GVA 0x%08X: %v", gva, err)
			continue
		}

		// Check if entire chunk is 0xFF (unmapped).
		allFF := true
		for i := 0; i < 256 && i < len(data); i++ {
			if data[i] != 0xFF {
				allFF = false
				break
			}
		}
		if allFF {
			continue
		}

		// Search for UTF-16LE gamertag patterns.
		for name, pattern := range patterns {
			for i := 0; i <= len(data)-len(pattern); i++ {
				if bytes.Equal(data[i:i+len(pattern)], pattern) {
					hitGVA := gva + uint32(i)
					start := max(0, i-64)
					end := min(len(data), i+len(pattern)+128)
					fmt.Printf("\n=== FOUND UTF16 %q at GVA 0x%08X (phys 0x%06X) ===\n",
						name, hitGVA, hitGVA-0x80000000)
					hexDump(data[start:end], gva+uint32(start))
				}
			}
		}

		// Search for ASCII map path patterns.
		for name, pattern := range asciiPatterns {
			for i := 0; i <= len(data)-len(pattern); i++ {
				if bytes.Equal(data[i:i+len(pattern)], pattern) {
					hitGVA := gva + uint32(i)
					start := max(0, i-32)
					end := min(len(data), i+len(pattern)+128)
					fmt.Printf("\n=== FOUND ASCII %q at GVA 0x%08X (phys 0x%06X) ===\n",
						name, hitGVA, hitGVA-0x80000000)
					hexDump(data[start:end], gva+uint32(start))
				}
			}
		}
	}

	log.Println("scan complete")
}

// utf16le encodes a string as UTF-16LE bytes.
func utf16le(s string) []byte {
	b := make([]byte, len(s)*2)
	for i, c := range s {
		b[i*2] = byte(c)
		b[i*2+1] = 0
	}
	return b
}

func dumpFullRAM(inst *xemu.Instance, outPath string) {
	const startGVA = uint32(0x80000000)
	const totalSize = 55 << 20 // 55 MB (skip GPU region at end)
	const chunkSize = 1 << 20

	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	defer f.Close()

	log.Printf("dumping %d MB of Xbox RAM (GVA 0x%08X–0x%08X) to %s...",
		totalSize>>20, startGVA, startGVA+totalSize, outPath)

	for offset := uint32(0); offset < totalSize; offset += chunkSize {
		gva := startGVA + offset
		data, err := inst.Mem.ReadBytes(gva, chunkSize)
		if err != nil {
			// Write zeros for unreadable chunks to preserve offsets.
			data = make([]byte, chunkSize)
		}
		if _, err := f.Write(data); err != nil {
			log.Fatalf("write error at offset 0x%X: %v", offset, err)
		}
		fmt.Printf("\r  %d / %d MB", (offset+chunkSize)>>20, totalSize>>20)
	}
	fmt.Println()
	log.Printf("done — %s (%d bytes)", outPath, totalSize)
	log.Printf("file offset = GVA - 0x80000000 (e.g. GVA 0x83691880 = file offset 0x03691880)")
}

// hexDump prints a hex dump with GVA addresses.
func hexDump(data []byte, startGVA uint32) {
	for i := 0; i < len(data); i += 16 {
		end := min(i+16, len(data))
		row := data[i:end]

		// Address
		fmt.Printf("  %08X: ", startGVA+uint32(i))

		// Hex bytes
		for j, b := range row {
			if j == 8 {
				fmt.Print(" ")
			}
			fmt.Printf("%02X ", b)
		}
		// Pad if short row
		for j := len(row); j < 16; j++ {
			if j == 8 {
				fmt.Print(" ")
			}
			fmt.Print("   ")
		}

		// ASCII
		fmt.Print(" |")
		for _, b := range row {
			if b >= 0x20 && b <= 0x7E {
				fmt.Printf("%c", b)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println("|")
	}
}
