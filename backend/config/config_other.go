//go:build !windows

package config

import (
	"strings"
)

func isNetworkDrive(path string) bool {
	// On Unix-like systems, UNC paths aren't used in the same way.
	// But we can check if it starts with double slash as a fallback.
	return strings.HasPrefix(path, "//")
}
