//go:build !darwin

package paths

// resolveTranslocatedPath is a no-op on non-macOS platforms.
func resolveTranslocatedPath(appPath string) string {
	return ""
}
