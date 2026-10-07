//go:build windows

package health

import (
	"syscall"
	"unsafe"
)

var getDiskFreeExW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func discoLivre() float64 {
	var free, total, avail uint64
	ret, _, _ := getDiskFreeExW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("C:\\"))),
		uintptr(unsafe.Pointer(&free)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&avail)),
	)
	if ret == 0 {
		return 0
	}
	return float64(free) / (1024 * 1024 * 1024)
}
