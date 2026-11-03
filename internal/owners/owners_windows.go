//go:build windows

package owners

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	iphlpapi          = syscall.NewLazyDLL("iphlpapi.dll")
	procExtendedTable = iphlpapi.NewProc("GetExtendedTcpTable")

	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess = kernel32.NewProc("OpenProcess")
	procImageName   = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

const (
	afInet              = 2
	tcpTableOwnerPIDAll = 5
	queryLimitedInfo    = 0x1000
	freshFor            = 2 * time.Second
)

type Resolver struct {
	mu      sync.Mutex
	ports   map[uint16]uint32
	names   map[uint32]string
	fetched time.Time
}

func New() *Resolver {
	return &Resolver{names: map[uint32]string{}}
}

func (r *Resolver) Of(port uint16) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ports == nil || time.Since(r.fetched) > freshFor {
		if ports, err := tcpPorts(); err == nil {
			r.ports = ports
			r.fetched = time.Now()
		}
	}

	pid, ok := r.ports[port]
	if !ok {
		return "", false
	}

	name, ok := r.names[pid]
	if !ok {
		name = processName(pid)
		r.names[pid] = name
	}

	return name, name != ""
}

// tcpPorts maps every local tcp port to its owning pid. The table is asked for its
// size first, then its contents, the way the windows call wants it.
func tcpPorts() (map[uint16]uint32, error) {
	var size uint32

	procExtendedTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, afInet, tcpTableOwnerPIDAll, 0)
	if size == 0 {
		return nil, syscall.EINVAL
	}

	buf := make([]byte, size)

	ret, _, _ := procExtendedTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, afInet, tcpTableOwnerPIDAll, 0)
	if ret != 0 {
		return nil, syscall.Errno(ret)
	}

	rows := binary.LittleEndian.Uint32(buf[0:4])
	out := make(map[uint16]uint32, rows)

	// Each MIB_TCPROW_OWNER_PID is six dwords: state, local addr, local port, remote
	// addr, remote port, pid. The port sits in its dword in network byte order.
	const row = 24
	for i := uint32(0); i < rows; i++ {
		at := 4 + i*row
		if int(at)+row > len(buf) {
			break
		}

		port := binary.BigEndian.Uint16(buf[at+8 : at+10])
		pid := binary.LittleEndian.Uint32(buf[at+20 : at+24])
		out[port] = pid
	}

	return out, nil
}

// processName is the lowercased exe name behind a pid, empty when it cannot be read
// (a system pid, or one gone between the table and this call).
func processName(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(queryLimitedInfo, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))

	ret, _, _ := procImageName.Call(handle, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret == 0 {
		return ""
	}

	return strings.ToLower(filepath.Base(syscall.UTF16ToString(buf[:size])))
}
