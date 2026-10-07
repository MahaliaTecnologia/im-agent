//go:build !windows

package platform

import (
	"runtime"
	"strings"
)

func syscallArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return strings.ToUpper(runtime.GOARCH)
	}
}
