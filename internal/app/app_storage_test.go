package app

import (
	"NeoNect/internal/config"
	"NeoNect/internal/logger"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnsureStoragePath_EnvironmentGating(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		app := &App{
			Config: config.AppConfig{
				Environment:  os.Getenv("CRASHER_ENV"),
				DatabaseDir:  os.Getenv("CRASHER_DB_DIR"),
				MasterKeyDir: os.Getenv("CRASHER_DB_DIR"),
			},
			Logger: logger.New(true), // Minimal mock, might panic on Fatalf if uninitialized, but it's okay because we check for non-zero exit
		}
		// override GetStoragePath via NEONECT_STORAGE_ROOT
		app.ensureStoragePath()
		os.Exit(0)
	}

	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "storage")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	outsideDir := filepath.Join(tempDir, "outside")

	tests := []struct {
		name        string
		env         string
		target      string
		expectCrash bool
	}{
		{"16. production enforcement", "production", outsideDir, true},
		{"17. beta enforcement", "beta", outsideDir, true},
		{"18. development compatibility", "development", outsideDir, false},
		{"Valid path in production", "production", filepath.Join(root, "db"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestEnsureStoragePath_EnvironmentGating")
			cmd.Env = append(os.Environ(),
				"BE_CRASHER=1",
				"CRASHER_ENV="+tt.env,
				"CRASHER_DB_DIR="+tt.target,
				"NEONECT_STORAGE_ROOT="+root,
			)
			err := cmd.Run()
			crashed := err != nil

			if crashed != tt.expectCrash {
				t.Errorf("expected crash: %v, got crash: %v (err: %v)", tt.expectCrash, crashed, err)
			}
		})
	}
}
