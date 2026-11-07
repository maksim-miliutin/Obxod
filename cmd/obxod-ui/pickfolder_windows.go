//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	procSHBrowseForFolder   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")

	ole32             = syscall.NewLazyDLL("ole32.dll")
	procCoInitialize  = ole32.NewProc("CoInitializeEx")
	procCoTaskMemFree = ole32.NewProc("CoTaskMemFree")
)

type browseInfoW struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	lParam      uintptr
	image       int32
}

const (
	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040
	coinitApartment     = 0x2
)

// browseFolder opens the native pick-a-folder dialog, empty when cancelled. The
// new-style dialog needs an sta com apartment, so the goroutine keeps its thread.
func browseFolder() string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	procCoInitialize.Call(0, coinitApartment)

	title, _ := syscall.UTF16PtrFromString("Выберите папку")
	name := make([]uint16, syscall.MAX_PATH)
	info := browseInfoW{
		displayName: &name[0],
		title:       title,
		flags:       bifReturnOnlyFSDirs | bifNewDialogStyle,
	}

	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(pidl)

	path := make([]uint16, syscall.MAX_PATH)
	ok, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 {
		return ""
	}

	return syscall.UTF16ToString(path)
}
