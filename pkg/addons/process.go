//go:build windows
// +build windows

package addons

import (
	"sort"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procProcess32First = kernel32.NewProc("Process32FirstW")
	procProcess32Next  = kernel32.NewProc("Process32NextW")
)

const th32csSnapProcess = 0x00000002

// processEntry32 is PROCESSENTRY32W.
type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// Processes returns the executable names of the running processes (e.g.
// "Couatl64.exe"), sorted and without duplicates. Which names matter is up
// to the caller.
func Processes() ([]string, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(th32csSnapProcess, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snap)
	var e processEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	r, _, err := procProcess32First.Call(uintptr(snap), uintptr(unsafe.Pointer(&e)))
	if r == 0 {
		return nil, err
	}
	seen := map[string]bool{}
	for r != 0 {
		seen[syscall.UTF16ToString(e.ExeFile[:])] = true
		r, _, _ = procProcess32Next.Call(uintptr(snap), uintptr(unsafe.Pointer(&e)))
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}
