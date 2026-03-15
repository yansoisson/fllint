package paths

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AppPaths holds all resolved filesystem paths for the application.
type AppPaths struct {
	BinDir       string // Directory containing llama-server binary
	DataDir      string // Directory for conversations, uploads, config
	ModelsDir    string // Directory containing .gguf model files
	Translocated bool   // True if running from a macOS App Translocation path
}

// Resolve determines BinDir, DataDir, and ModelsDir using this priority:
//
//  1. Environment variables (FLLINT_BIN_DIR, FLLINT_DATA_DIR, FLLINT_MODELS_DIR)
//  2. Platform-specific bundle detection (.app on macOS, AppImage on Linux)
//  3. CWD-relative defaults (./bin, ./data, ./models)
//
// Individual env vars override only their respective path. For example,
// setting FLLINT_DATA_DIR while running inside a .app bundle uses the env
// var for DataDir but still resolves BinDir from the bundle.
func Resolve() AppPaths {
	// Start with CWD defaults
	p := AppPaths{
		BinDir:    "./bin",
		DataDir:   "./data",
		ModelsDir: "./models",
	}

	// Try platform-specific bundle detection
	mode := "cwd"
	if bundlePaths, ok := detectBundle(); ok {
		p = bundlePaths
		mode = "bundle"
	}

	// Environment variable overrides (highest priority, per-path)
	if v := os.Getenv("FLLINT_BIN_DIR"); v != "" {
		p.BinDir = v
		mode = "env"
	}
	if v := os.Getenv("FLLINT_DATA_DIR"); v != "" {
		p.DataDir = v
		mode = "env"
	}
	if v := os.Getenv("FLLINT_MODELS_DIR"); v != "" {
		p.ModelsDir = v
		mode = "env"
	}

	// Make all paths absolute to avoid CWD-dependent behavior
	p.BinDir = makeAbs(p.BinDir)
	p.DataDir = makeAbs(p.DataDir)
	p.ModelsDir = makeAbs(p.ModelsDir)

	log.Printf("Path resolution (%s): bin=%s, data=%s, models=%s",
		mode, p.BinDir, p.DataDir, p.ModelsDir)

	return p
}

// detectBundle checks if the executable is running inside a platform-specific
// application bundle and returns appropriate paths if so.
func detectBundle() (AppPaths, bool) {
	switch runtime.GOOS {
	case "darwin":
		return detectDarwinBundle()
	// case "linux":
	//     return detectLinuxAppImage()
	default:
		return AppPaths{}, false
	}
}

// detectDarwinBundle checks if the running binary is inside a macOS .app bundle.
//
// Expected structure:
//
//	Fllint/
//	  Fllint.app/
//	    Contents/
//	      MacOS/
//	        fllint           <-- os.Executable() points here
//	      Resources/
//	        bin/
//	          llama-server
//	  Data/
//	    models/
//	    conversations/
//
// The "bundle root" is the parent of Fllint.app/ (the Fllint/ folder).
func detectDarwinBundle() (AppPaths, bool) {
	exe, err := os.Executable()
	if err != nil {
		return AppPaths{}, false
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return AppPaths{}, false
	}

	return parseDarwinBundle(exe)
}

// parseDarwinBundle resolves paths from a macOS .app executable path.
// Separated from detectDarwinBundle for testability.
//
// Handles macOS App Translocation: when a quarantined app is launched,
// macOS copies the .app to a temporary read-only path. The binary and
// resources inside the .app are accessible at the translocated path,
// but the Data/ folder remains at the original location on disk.
func parseDarwinBundle(exePath string) (AppPaths, bool) {
	if !strings.Contains(exePath, ".app/Contents/MacOS/") {
		return AppPaths{}, false
	}

	// Walk up: fllint → MacOS → Contents → Fllint.app → Fllint (bundle root)
	macosDir := filepath.Dir(exePath)     // .../Fllint.app/Contents/MacOS
	contentsDir := filepath.Dir(macosDir) // .../Fllint.app/Contents
	appDir := filepath.Dir(contentsDir)   // .../Fllint.app

	// BinDir always uses the current (possibly translocated) path,
	// since the binaries are inside the .app and accessible there.
	binDir := filepath.Join(contentsDir, "Resources", "bin")

	// For DataDir, resolve the original path if the app is translocated.
	// The Data/ folder lives alongside the .app on disk, not inside it,
	// so it won't exist at the translocated temporary path.
	dataAppDir := appDir
	translocated := false
	if original := resolveTranslocatedPath(appDir); original != "" {
		log.Printf("App Translocation detected: data resolved to %s", filepath.Dir(original))
		dataAppDir = original
		translocated = true
	}
	bundleRoot := filepath.Dir(dataAppDir)

	return AppPaths{
		BinDir:       binDir,
		DataDir:      filepath.Join(bundleRoot, "Data"),
		ModelsDir:    filepath.Join(bundleRoot, "Data", "models"),
		Translocated: translocated,
	}, true
}

func makeAbs(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
