// localproof watches player movement data from two sources over time:
//
//	(a) biped object — move_forward (+0x228) and move_left (+0x22C)
//	    Confirmed synced over LAN for all players.
//
//	(b) update queue — forward (+0x14) and left (+0x18), per player slot
//	    Documented as "local player only".
//
// Run it, push some buttons, and watch whether the UQ columns track the biped
// columns for the remote player slots.  If they match, the update queue IS
// populated for remote players.
//
// Run as root:
//
//	sudo go run ./cmd/localproof
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"time"
	"unicode/utf16"

	"xemu-cartographer/internal/scraper/haloce"
	"xemu-cartographer/internal/xemu"
)

const (
	instName = "xemu-host-01"
	qmpSock  = "/var/lib/xemu/xemu-host-01/qmp/xemu-host-01.sock"

	pollDuration = 15 * time.Second
	pollInterval = 33 * time.Millisecond
	heartbeat    = 3 * time.Second // print "still watching" if nothing changes

	addrUpdateQueuePtr   uint32 = 0x2E8870
	addrPlayerControlPtr uint32 = 0x276794
)

type playerState struct {
	alive       bool
	bFwd, bLeft float32 // biped move_forward/move_left (confirmed synced)
	uqFwd       float32 // update queue forward
	uqLeft      float32 // update queue left
	uqYaw       float32 // update queue desired_yaw
	pcYaw       float32 // player control desired_yaw
	bAimX       float32 // biped aiming_vector x (reference only)
}

type playerMeta struct {
	name       string
	localIndex int16 // -1 = remote (real Xbox), 0+ = local (xemu dummy)
}

func main() {
	gvas := make([]uint32, len(haloce.AllLowGVAs), len(haloce.AllLowGVAs)+2)
	copy(gvas, haloce.AllLowGVAs)
	gvas = append(gvas, addrUpdateQueuePtr, addrPlayerControlPtr)

	inst := &xemu.Instance{Name: instName, QMPSock: qmpSock}
	if err := inst.Init(gvas); err != nil {
		log.Fatalf("init: %v", err)
	}
	log.Printf("%s: ready — polling for %.0fs\n", instName, pollDuration.Seconds())

	mem := inst.Mem

	// --- Static layout (read once) ---
	pdaBase, _ := inst.DerefLowPtr(haloce.AddrPlayerDatumArrayPtr)
	elemSize, _ := mem.ReadU16(pdaBase + haloce.OffPDAElementSize)
	currentCount, _ := mem.ReadU16(pdaBase + haloce.OffPDACurrentCount)
	firstElement, _ := mem.ReadU32(pdaBase + haloce.OffPDAFirstElement)

	ohdBase, _ := inst.DerefLowPtr(haloce.AddrObjectHeaderDatumPtr)
	var objElemSize uint16
	var objHeaderFirst uint32
	if ohdBase >= 0x80000000 {
		objElemSize, _ = mem.ReadU16(ohdBase + haloce.OffOHDElementSize)
		objHeaderFirst, _ = mem.ReadU32(ohdBase + haloce.OffOHDFirstElement)
	}

	uqBase, _ := inst.DerefLowPtr(addrUpdateQueuePtr)
	pcBase, _ := inst.DerefLowPtr(addrPlayerControlPtr)

	// Collect player metadata and print roster.
	meta := make(map[uint16]playerMeta)
	for i := uint16(0); i < currentCount; i++ {
		playerBase := firstElement + uint32(i)*uint32(elemSize)
		nb, err := mem.ReadBytes(playerBase+haloce.OffPlrName, 24)
		if err != nil || (nb[0] == 0 && nb[1] == 0) {
			continue
		}
		li, _ := mem.ReadS16(playerBase + haloce.OffPlrLocalIndex)
		meta[i] = playerMeta{name: decodeUTF16LE(nb), localIndex: li}
	}

	fmt.Printf("\nRoster (%d slots):\n", len(meta))
	for i := uint16(0); i < currentCount; i++ {
		m, ok := meta[i]
		if !ok {
			continue
		}
		locality := "remote"
		if m.localIndex >= 0 {
			locality = fmt.Sprintf("local=%d", m.localIndex)
		}
		fmt.Printf("  [%d] %-14s  %s\n", i, m.name, locality)
	}

	fmt.Printf("\nColumns: B=biped(synced)  UQ=update-queue  PC=player-control\n")
	fmt.Printf("%-8s  %-3s  %-12s  %-6s  %-5s  %8s %8s    %8s %8s %8s    %8s  %8s\n",
		"elapsed", "IDX", "name", "local", "alive",
		"B_FWD", "B_LEFT",
		"UQ_FWD", "UQ_LEFT", "UQ_YAW",
		"PC_YAW", "B_AIMX")
	fmt.Println("--------  ---  ------------  ------  -----  --------  --------    --------  --------  --------    --------  --------")

	prev := make(map[uint16]playerState)
	start := time.Now()
	deadline := start.Add(pollDuration)
	lastPrint := time.Now()

	for time.Now().Before(deadline) {
		// Re-read object header first_element every tick (table rearranges).
		if ohdBase >= 0x80000000 {
			objHeaderFirst, _ = mem.ReadU32(ohdBase + haloce.OffOHDFirstElement)
		}

		changed := false
		for i := uint16(0); i < currentCount; i++ {
			if _, ok := meta[i]; !ok {
				continue
			}
			playerBase := firstElement + uint32(i)*uint32(elemSize)
			handle, _ := mem.ReadS32(playerBase + haloce.OffPlrObjectHandle)
			alive := handle != -1

			var bFwd, bLeft, bAimX float32
			if alive && objHeaderFirst >= 0x80000000 && objElemSize > 0 {
				objIdx := uint32(handle) & 0xFFFF
				objDataAddr, _ := mem.ReadU32(objHeaderFirst + objIdx*uint32(objElemSize) + haloce.OffObjEntryDataAddr)
				if objDataAddr >= 0x80000000 {
					bFwd, _ = mem.ReadF32(objDataAddr + 0x228)
					bLeft, _ = mem.ReadF32(objDataAddr + 0x22C)
					bAimX, _ = mem.ReadF32(objDataAddr + 0x1EC)
				}
			}

			var uqFwd, uqLeft, uqYaw float32
			if uqBase >= 0x80000000 {
				slot := uqBase + 0x34 + uint32(i)*0x28
				uqYaw, _ = mem.ReadF32(slot + 0x0C)
				uqFwd, _ = mem.ReadF32(slot + 0x14)
				uqLeft, _ = mem.ReadF32(slot + 0x18)
			}

			var pcYaw float32
			if pcBase >= 0x80000000 {
				pcYaw, _ = mem.ReadF32(pcBase + uint32(i)*64 + 0x1C)
			}

			cur := playerState{alive, bFwd, bLeft, uqFwd, uqLeft, uqYaw, pcYaw, bAimX}
			p := prev[i]
			if cur == p {
				continue
			}
			prev[i] = cur
			changed = true

			m := meta[i]
			locality := "remote"
			if m.localIndex >= 0 {
				locality = fmt.Sprintf("loc=%d", m.localIndex)
			}
			elapsed := time.Since(start).Truncate(time.Millisecond)
			fmt.Printf("%-8s  %-3d  %-12s  %-6s  %-5v  %8.3f %8.3f    %8.3f %8.3f %8.3f    %8.3f  %8.3f\n",
				elapsed, i, m.name, locality, alive,
				bFwd, bLeft,
				uqFwd, uqLeft, uqYaw,
				pcYaw, bAimX)
		}

		if !changed && time.Since(lastPrint) >= heartbeat {
			fmt.Printf("[%s]  no changes\n", time.Since(start).Truncate(time.Second))
			lastPrint = time.Now()
		} else if changed {
			lastPrint = time.Now()
		}

		time.Sleep(pollInterval)
	}

	fmt.Printf("\ndone (%.0fs elapsed)\n", time.Since(start).Seconds())
}

func decodeUTF16LE(b []byte) string {
	u16s := make([]uint16, len(b)/2)
	for i := range u16s {
		u16s[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	for i, c := range u16s {
		if c == 0 {
			u16s = u16s[:i]
			break
		}
	}
	return string(utf16.Decode(u16s))
}
