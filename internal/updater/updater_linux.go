//go:build linux

package updater

import "fmt"

// CheckForUpdate is a no-op on Linux. Auto-updates are only available on macOS.
func CheckForUpdate() error {
	return fmt.Errorf("auto-updates are only available on macOS")
}

// HelperExists always returns false on Linux.
func HelperExists() bool {
	return false
}
