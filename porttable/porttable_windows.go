// Package porttable talks directly to the Windows IP Helper API
// (iphlpapi.dll) to enumerate TCP connections/listeners, exactly the
// same underlying call netstat itself uses. No shelling out, no text
// parsing.
package porttable

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PortEntry is one row of the TCP table, translated into Go-friendly types.
type PortEntry struct {
	LocalPort  uint16
	LocalAddr  string
	State      string
	RemotePort uint16
	RemoteAddr string
	PID        uint32
}

// Raw struct layout matching MIB_TCPROW_OWNER_PID from the Windows SDK.
// Field order and sizes must match the C struct exactly since we're
// reading this straight out of a buffer the kernel filled in.
type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32 // only low 16 bits used, stored big-endian
	RemoteAddr uint32
	RemotePort uint32 // only low 16 bits used, stored big-endian
	OwningPID  uint32
}

const (
	afINET              = 2 // AF_INET
	tcpTableOwnerPIDAll = 5 // TCP_TABLE_OWNER_PID_ALL

	tcpStateListen = 2 // MIB_TCP_STATE_LISTEN
)

var tcpStateNames = map[uint32]string{
	1:  "CLOSED",
	2:  "LISTENING",
	3:  "SYN_SENT",
	4:  "SYN_RCVD",
	5:  "ESTABLISHED",
	6:  "FIN_WAIT1",
	7:  "FIN_WAIT2",
	8:  "CLOSE_WAIT",
	9:  "CLOSING",
	10: "LAST_ACK",
	11: "TIME_WAIT",
	12: "DELETE_TCB",
}

var (
	modIPHlpAPI        = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTCP = modIPHlpAPI.NewProc("GetExtendedTcpTable")
)

// portFromNetworkOrder extracts the 16-bit port from the low bits of a
// DWORD, then swaps byte order. Windows stores the port big-endian
// inside a little-endian DWORD, so a straight cast gives you garbage.
func portFromNetworkOrder(raw uint32) uint16 {
	b0 := byte(raw)
	b1 := byte(raw >> 8)
	return uint16(b0)<<8 | uint16(b1)
}

func ipv4String(raw uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24))
}

// GetTCPTable fetches the full current TCP table from the OS in one
// syscall. There is no API to ask for a single port — the kernel only
// exposes a bulk snapshot, so callers filter the returned slice
// themselves (see Filter).
func GetTCPTable() ([]PortEntry, error) {
	var size uint32

	// First call with a nil buffer: the API fills `size` with the
	// buffer length it actually needs, then returns
	// ERROR_INSUFFICIENT_BUFFER. This is the standard two-call Win32
	// pattern for variable-length results.
	ret, _, _ := procGetExtendedTCP.Call(
		0,
		uintptr(unsafe.Pointer(&size)),
		0, // bOrder
		uintptr(afINET),
		uintptr(tcpTableOwnerPIDAll),
		0,
	)
	const errInsufficientBuffer = 122
	if ret != errInsufficientBuffer {
		return nil, fmt.Errorf("unexpected return sizing TCP table: %d", ret)
	}

	buf := make([]byte, size)
	ret, _, _ = procGetExtendedTCP.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(afINET),
		uintptr(tcpTableOwnerPIDAll),
		0,
	)
	if ret != 0 {
		return nil, fmt.Errorf("GetExtendedTcpTable failed: %d", ret)
	}

	numEntries := *(*uint32)(unsafe.Pointer(&buf[0]))
	rowSize := unsafe.Sizeof(mibTCPRowOwnerPID{})
	tableStart := unsafe.Pointer(&buf[4]) // skip the dwNumEntries header

	entries := make([]PortEntry, 0, numEntries)
	for i := uint32(0); i < numEntries; i++ {
		rowPtr := unsafe.Add(tableStart, uintptr(i)*rowSize)
		row := (*mibTCPRowOwnerPID)(rowPtr)

		state, ok := tcpStateNames[row.State]
		if !ok {
			state = "UNKNOWN"
		}

		entries = append(entries, PortEntry{
			LocalPort:  portFromNetworkOrder(row.LocalPort),
			LocalAddr:  ipv4String(row.LocalAddr),
			State:      state,
			RemotePort: portFromNetworkOrder(row.RemotePort),
			RemoteAddr: ipv4String(row.RemoteAddr),
			PID:        row.OwningPID,
		})
	}

	return entries, nil
}

// FilterByPort returns only entries bound to the given local port.
func FilterByPort(entries []PortEntry, port uint16) []PortEntry {
	var out []PortEntry
	for _, e := range entries {
		if e.LocalPort == port {
			out = append(out, e)
		}
	}
	return out
}

// FilterListening returns only entries actively listening for
// connections (used by --all, since nobody cares about your TIME_WAIT
// sockets from an hour ago).
func FilterListening(entries []PortEntry) []PortEntry {
	var out []PortEntry
	for _, e := range entries {
		if e.State == "LISTENING" {
			out = append(out, e)
		}
	}
	return out
}