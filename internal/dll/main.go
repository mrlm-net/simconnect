//go:build windows
// +build windows

package dll

import (
	"sync"
	"syscall"
)

func New(path string) *DLL {
	return &DLL{
		binary:     syscall.NewLazyDLL(path),
		path:       path,
		procedures: make(map[string]*syscall.LazyProc),
	}
}

type DLL struct {
	binary     *syscall.LazyDLL
	path       string
	procedures map[string]*syscall.LazyProc
	sync       sync.Mutex
}

func (dll *DLL) LoadProcedure(name string) *syscall.LazyProc {
	dll.sync.Lock()
	defer dll.sync.Unlock()
	if _, exists := dll.procedures[name]; !exists {
		dll.procedures[name] = dll.binary.NewProc(name)
	}
	return dll.procedures[name]
}

func (dll *DLL) Path() string {
	return dll.path
}
