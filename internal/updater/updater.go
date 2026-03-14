//go:build darwin

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CheckForUpdate launches the sparkle-helper binary to trigger
// Sparkle's native update dialog. Returns an error if the helper
// is not found or cannot be launched.
func CheckForUpdate() error {
	helperPath, err := helperPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(helperPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch sparkle-helper: %w", err)
	}

	// Reap the process in background to avoid zombies
	go cmd.Wait()
	return nil
}

// HelperExists checks if the sparkle-helper binary is available
// next to the running executable (inside .app/Contents/MacOS/).
func HelperExists() bool {
	p, err := helperPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

func helperPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine executable path: %w", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)
	return filepath.Join(filepath.Dir(exe), "sparkle-helper"), nil
}
