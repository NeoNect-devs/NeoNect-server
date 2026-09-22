package shutdown_test

import (
	"NeoNect/test/harness"
	"os"
	"testing"
)

func TestShutdown_EnvCleanup(t *testing.T) {
	// Pre-set some environment variables
	os.Setenv("NEONECT_ENV", "production")
	os.Setenv("NEONECT_BIND_ADDR", "0.0.0.0:8080")

	t.Run("Subtest", func(st *testing.T) {
		_ = harness.Setup(st)
		if os.Getenv("NEONECT_ENV") != "development" {
			st.Errorf("Expected NEONECT_ENV to be development inside harness")
		}
	}) // Subtest finishes, cleanups run

	if os.Getenv("NEONECT_ENV") != "production" {
		t.Errorf("Expected NEONECT_ENV to be restored to production, got: %s", os.Getenv("NEONECT_ENV"))
	}
	if os.Getenv("NEONECT_BIND_ADDR") != "0.0.0.0:8080" {
		t.Errorf("Expected NEONECT_BIND_ADDR to be restored")
	}

	// Clean up
	os.Unsetenv("NEONECT_ENV")
	os.Unsetenv("NEONECT_BIND_ADDR")
}
