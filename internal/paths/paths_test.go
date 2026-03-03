package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDarwinBundle(t *testing.T) {
	tests := []struct {
		name    string
		exePath string
		wantOK  bool
		wantBin string
		wantDat string
		wantMod string
	}{
		{
			name:    "valid app bundle",
			exePath: "/Applications/Fllint/Fllint.app/Contents/MacOS/fllint",
			wantOK:  true,
			wantBin: "/Applications/Fllint/Fllint.app/Contents/Resources/bin",
			wantDat: "/Applications/Fllint/Data",
			wantMod: "/Applications/Fllint/Data/models",
		},
		{
			name:    "bundle on desktop",
			exePath: "/Users/alice/Desktop/Fllint/Fllint.app/Contents/MacOS/fllint",
			wantOK:  true,
			wantBin: "/Users/alice/Desktop/Fllint/Fllint.app/Contents/Resources/bin",
			wantDat: "/Users/alice/Desktop/Fllint/Data",
			wantMod: "/Users/alice/Desktop/Fllint/Data/models",
		},
		{
			name:    "bundle on external drive",
			exePath: "/Volumes/USB/Fllint/Fllint.app/Contents/MacOS/fllint",
			wantOK:  true,
			wantBin: "/Volumes/USB/Fllint/Fllint.app/Contents/Resources/bin",
			wantDat: "/Volumes/USB/Fllint/Data",
			wantMod: "/Volumes/USB/Fllint/Data/models",
		},
		{
			name:    "not inside app bundle — plain binary",
			exePath: "/usr/local/bin/fllint",
			wantOK:  false,
		},
		{
			name:    "not inside app bundle — go run temp dir",
			exePath: "/var/folders/xx/yy/T/go-build123/exe/main",
			wantOK:  false,
		},
		{
			name:    "partial match — .app in path but not Contents/MacOS",
			exePath: "/Applications/Fllint.app/fllint",
			wantOK:  false,
		},
		{
			name:    "app bundle with spaces in path",
			exePath: "/Users/alice/My Apps/Fllint/Fllint.app/Contents/MacOS/fllint",
			wantOK:  true,
			wantBin: "/Users/alice/My Apps/Fllint/Fllint.app/Contents/Resources/bin",
			wantDat: "/Users/alice/My Apps/Fllint/Data",
			wantMod: "/Users/alice/My Apps/Fllint/Data/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDarwinBundle(tt.exePath)
			if ok != tt.wantOK {
				t.Fatalf("parseDarwinBundle(%q) ok = %v, want %v", tt.exePath, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.BinDir != tt.wantBin {
				t.Errorf("BinDir = %q, want %q", got.BinDir, tt.wantBin)
			}
			if got.DataDir != tt.wantDat {
				t.Errorf("DataDir = %q, want %q", got.DataDir, tt.wantDat)
			}
			if got.ModelsDir != tt.wantMod {
				t.Errorf("ModelsDir = %q, want %q", got.ModelsDir, tt.wantMod)
			}
		})
	}
}

func TestMakeAbs(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Relative path should become absolute
	got := makeAbs("./data")
	want := filepath.Join(cwd, "data")
	if got != want {
		t.Errorf("makeAbs(\"./data\") = %q, want %q", got, want)
	}

	// Already absolute path should stay the same
	got = makeAbs("/tmp/fllint/data")
	if got != "/tmp/fllint/data" {
		t.Errorf("makeAbs(\"/tmp/fllint/data\") = %q, want /tmp/fllint/data", got)
	}
}

func TestResolveWithEnvVars(t *testing.T) {
	// Save and clear any existing env vars
	envVars := []string{"FLLINT_BIN_DIR", "FLLINT_DATA_DIR", "FLLINT_MODELS_DIR"}
	saved := make(map[string]string)
	for _, key := range envVars {
		saved[key] = os.Getenv(key)
	}
	t.Cleanup(func() {
		for _, key := range envVars {
			if saved[key] != "" {
				os.Setenv(key, saved[key])
			} else {
				os.Unsetenv(key)
			}
		}
	})

	t.Run("env vars override all paths", func(t *testing.T) {
		os.Setenv("FLLINT_BIN_DIR", "/custom/bin")
		os.Setenv("FLLINT_DATA_DIR", "/custom/data")
		os.Setenv("FLLINT_MODELS_DIR", "/custom/models")

		p := Resolve()

		if p.BinDir != "/custom/bin" {
			t.Errorf("BinDir = %q, want /custom/bin", p.BinDir)
		}
		if p.DataDir != "/custom/data" {
			t.Errorf("DataDir = %q, want /custom/data", p.DataDir)
		}
		if p.ModelsDir != "/custom/models" {
			t.Errorf("ModelsDir = %q, want /custom/models", p.ModelsDir)
		}
	})

	t.Run("partial env var override", func(t *testing.T) {
		os.Unsetenv("FLLINT_BIN_DIR")
		os.Setenv("FLLINT_DATA_DIR", "/override/data")
		os.Unsetenv("FLLINT_MODELS_DIR")

		p := Resolve()

		// DataDir should be the env var value
		if p.DataDir != "/override/data" {
			t.Errorf("DataDir = %q, want /override/data", p.DataDir)
		}
		// BinDir and ModelsDir should be absolute CWD-based defaults
		// (since we're not in a .app bundle during tests)
		if !filepath.IsAbs(p.BinDir) {
			t.Errorf("BinDir should be absolute, got %q", p.BinDir)
		}
		if !filepath.IsAbs(p.ModelsDir) {
			t.Errorf("ModelsDir should be absolute, got %q", p.ModelsDir)
		}
	})

	t.Run("no env vars — CWD defaults made absolute", func(t *testing.T) {
		os.Unsetenv("FLLINT_BIN_DIR")
		os.Unsetenv("FLLINT_DATA_DIR")
		os.Unsetenv("FLLINT_MODELS_DIR")

		p := Resolve()

		if !filepath.IsAbs(p.BinDir) {
			t.Errorf("BinDir should be absolute, got %q", p.BinDir)
		}
		if !filepath.IsAbs(p.DataDir) {
			t.Errorf("DataDir should be absolute, got %q", p.DataDir)
		}
		if !filepath.IsAbs(p.ModelsDir) {
			t.Errorf("ModelsDir should be absolute, got %q", p.ModelsDir)
		}

		cwd, _ := os.Getwd()
		wantBin := filepath.Join(cwd, "bin")
		wantData := filepath.Join(cwd, "data")
		wantModels := filepath.Join(cwd, "models")

		if p.BinDir != wantBin {
			t.Errorf("BinDir = %q, want %q", p.BinDir, wantBin)
		}
		if p.DataDir != wantData {
			t.Errorf("DataDir = %q, want %q", p.DataDir, wantData)
		}
		if p.ModelsDir != wantModels {
			t.Errorf("ModelsDir = %q, want %q", p.ModelsDir, wantModels)
		}
	})
}
