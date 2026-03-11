// prove is a proof-of-concept that reads Halo CE game_time ticks from a running
// xemu instance via /proc/<pid>/mem. QMP is used once at startup to translate
// guest virtual addresses to host virtual addresses.
//
// Run as root (needs /proc/<pid>/mem read access):
//
//	sudo go run ./cmd/prove
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	instance = "xemu-host-02"
	qmpSock  = "/var/lib/xemu/xemu-host-02/qmp/xemu-host-02.sock"

	// addrGameTimeGlobalsPtr is a guest VA holding a pointer to the GameTimeGlobals struct.
	// Dereference it to get the struct address, then read +0x0C for game_time ticks.
	addrGameTimeGlobalsPtr = uint32(0x2F8CA0)
)

// findXemuPID scans /proc for the AppRun process belonging to instance.
func findXemuPID() (int, error) {
	entries, _ := filepath.Glob("/proc/*/cmdline")
	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}
		// cmdline fields are NUL-separated; first field is the executable
		fields := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if len(fields) == 0 {
			continue
		}
		exe := fields[0]
		cmdline := strings.Join(fields, " ")
		if strings.HasSuffix(exe, "AppRun") && strings.Contains(cmdline, instance+".sock") {
			parts := strings.Split(entry, "/")
			if len(parts) >= 3 {
				if pid, err := strconv.Atoi(parts[2]); err == nil {
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no AppRun process found for %s", instance)
}

// qmpClient holds an open QMP connection after the handshake.
type qmpClient struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func newQMPClient() (*qmpClient, error) {
	conn, err := net.Dial("unix", qmpSock)
	if err != nil {
		return nil, fmt.Errorf("connect QMP: %w", err)
	}
	c := &qmpClient{conn: conn, scanner: bufio.NewScanner(conn)}

	// Read greeting banner.
	if !c.scanner.Scan() {
		conn.Close()
		return nil, fmt.Errorf("no QMP banner")
	}

	// Negotiate capabilities.
	fmt.Fprintln(conn, `{"execute":"qmp_capabilities"}`)
	if !c.scanner.Scan() {
		conn.Close()
		return nil, fmt.Errorf("no capabilities response")
	}
	return c, nil
}

func (c *qmpClient) Close() { c.conn.Close() }

// hmpCommand sends a Human Monitor Protocol command and returns the return string.
func (c *qmpClient) hmpCommand(cmd string) (string, error) {
	req := fmt.Sprintf(`{"execute":"human-monitor-command","arguments":{"command-line":%q}}`, cmd)
	fmt.Fprintln(c.conn, req)
	if !c.scanner.Scan() {
		return "", fmt.Errorf("no response for %q", cmd)
	}
	var resp struct{ Return string }
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parse response for %q: %w", cmd, err)
	}
	return strings.TrimSpace(resp.Return), nil
}

// parseHexSuffix extracts the last whitespace-separated token and parses it as hex.
func parseHexSuffix(s string) (uint64, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty response: %q", s)
	}
	v, err := strconv.ParseUint(fields[len(fields)-1], 0, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", fields[len(fields)-1], err)
	}
	return v, nil
}

// gpa2hva translates a guest physical address to a host virtual address via QMP.
func (c *qmpClient) gpa2hva(gpa uint64) (uint64, error) {
	ret, err := c.hmpCommand(fmt.Sprintf("gpa2hva 0x%x", gpa))
	if err != nil {
		return 0, err
	}
	// "Host virtual address for 0x... (...) is 0x..."
	return parseHexSuffix(ret)
}

// gva2gpa translates a guest virtual address to a guest physical address via QMP.
func (c *qmpClient) gva2gpa(gva uint32) (uint64, error) {
	ret, err := c.hmpCommand(fmt.Sprintf("gva2gpa 0x%x", gva))
	if err != nil {
		return 0, err
	}
	// "Physical address for 0x... is 0x..."
	return parseHexSuffix(ret)
}

// translateGVA translates a guest virtual address to a host virtual address.
// For addresses >= 0x80000000: use pre-computed base (base + VA - 0x80000000).
// For addresses  < 0x80000000: use gva2gpa + gpa2hva (proper two-step QMP translation).
func translateGVA(c *qmpClient, base uint64, gva uint32) (int64, error) {
	if gva >= 0x80000000 {
		return int64(base) + int64(gva-0x80000000), nil
	}
	gpa, err := c.gva2gpa(gva)
	if err != nil {
		return 0, fmt.Errorf("gva2gpa 0x%x: %w", gva, err)
	}
	hva, err := c.gpa2hva(gpa)
	if err != nil {
		return 0, fmt.Errorf("gpa2hva 0x%x: %w", gpa, err)
	}
	return int64(hva), nil
}

func readU32(fd uintptr, addr int64) (uint32, error) {
	buf := make([]byte, 4)
	n, err := syscall.Pread(int(fd), buf, addr)
	if err != nil {
		return 0, fmt.Errorf("pread at 0x%x: %w", addr, err)
	}
	if n != 4 {
		return 0, fmt.Errorf("short read at 0x%x: got %d bytes", addr, n)
	}
	return binary.LittleEndian.Uint32(buf), nil
}

func main() {
	pid, err := findXemuPID()
	if err != nil {
		fmt.Fprintln(os.Stderr, "find PID:", err)
		os.Exit(1)
	}
	fmt.Printf("xemu PID:      %d\n", pid)

	qmp, err := newQMPClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "QMP:", err)
		os.Exit(1)
	}

	// Get Xbox RAM base via gpa2hva 0x0.
	base, err := qmp.gpa2hva(0x0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gpa2hva base:", err)
		os.Exit(1)
	}
	fmt.Printf("Xbox RAM base: 0x%x\n", base)

	// Translate GameTimeGlobalsPtr (low address) using gva2gpa + gpa2hva.
	gtgptrHVA, err := translateGVA(qmp, base, addrGameTimeGlobalsPtr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "translate GameTimeGlobalsPtr:", err)
		os.Exit(1)
	}
	fmt.Printf("GameTimeGlobalsPtr HVA: 0x%x\n", gtgptrHVA)

	qmp.Close() // done with QMP

	// Open /proc/<pid>/mem for pread.
	f, err := os.OpenFile(fmt.Sprintf("/proc/%d/mem", pid), os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open mem:", err)
		os.Exit(1)
	}
	defer f.Close()
	fd := f.Fd()

	// Read the pointer at GameTimeGlobalsPtr to get the struct address.
	gtgAddr, err := readU32(fd, gtgptrHVA)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read GameTimeGlobalsPtr:", err)
		os.Exit(1)
	}
	fmt.Printf("GameTimeGlobals addr:   0x%x\n\n", gtgAddr)

	if gtgAddr < 0x80000000 {
		fmt.Fprintf(os.Stderr, "unexpected GameTimeGlobals addr 0x%x (expected >= 0x80000000)\n", gtgAddr)
		os.Exit(1)
	}

	// game_time is at GameTimeGlobals + 0x0C.
	gameTimeHVA := int64(base) + int64(gtgAddr-0x80000000) + 0x0C

	fmt.Println("Reading game_time (30 ticks/sec) — Ctrl+C to stop")
	fmt.Println()

	prev := uint32(0xFFFFFFFF)
	for {
		val, err := readU32(fd, gameTimeHVA)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		if val != prev {
			fmt.Printf("tick=%-8d  t=%6.2fs\n", val, float64(val)/30.0)
			prev = val
		}
		time.Sleep(33 * time.Millisecond)
	}
}
