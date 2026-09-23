package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateStoragePath(t *testing.T) {
	tempDir := t.TempDir()

	realRoot := filepath.Join(tempDir, "real_storage")
	if err := os.Mkdir(realRoot, 0755); err != nil {
		t.Fatal(err)
	}

	outsideDir := filepath.Join(tempDir, "outside")
	if err := os.Mkdir(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create symlink for symlinked storage root
	symlinkedRoot := filepath.Join(tempDir, "symlinked_storage")
	if err := os.Symlink(realRoot, symlinkedRoot); err != nil {
		t.Fatal(err)
	}

	// Existing symlink inside root pointing to inside root
	insideTargetDir := filepath.Join(realRoot, "inside_target")
	if err := os.Mkdir(insideTargetDir, 0755); err != nil {
		t.Fatal(err)
	}
	symlinkInsideToInside := filepath.Join(realRoot, "link_to_inside")
	if err := os.Symlink(insideTargetDir, symlinkInsideToInside); err != nil {
		t.Fatal(err)
	}

	// Existing symlink inside root pointing to outside root
	symlinkInsideToOutside := filepath.Join(realRoot, "link_to_outside")
	if err := os.Symlink(outsideDir, symlinkInsideToOutside); err != nil {
		t.Fatal(err)
	}

	// Nested symlinked parent escaping root
	nestedDir := filepath.Join(realRoot, "nested")
	if err := os.Mkdir(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	symlinkNestedToOutside := filepath.Join(nestedDir, "link_to_outside")
	if err := os.Symlink(outsideDir, symlinkNestedToOutside); err != nil {
		t.Fatal(err)
	}

	// Sibling prefix setup
	realRootSibling := filepath.Join(tempDir, "real_storage2")
	if err := os.Mkdir(realRootSibling, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		target    string
		root      string
		wantError bool
	}{
		// Basic paths
		{"1. valid child path", filepath.Join(realRoot, "db"), realRoot, false},
		{"2. valid nested child path", filepath.Join(realRoot, "a", "b", "c"), realRoot, false},
		{"3. exact root path", realRoot, realRoot, false},

		// Traversals
		{"4. ../ traversal", filepath.Join(realRoot, "..", "outside"), realRoot, true},
		{"5. normalized traversal", filepath.Join(realRoot, "nested", "..", "..", "outside"), realRoot, true},

		// Bypasses
		{"6. sibling-prefix bypass", realRootSibling, realRoot, true},
		{"7. absolute outside path", outsideDir, realRoot, true},
		{"8. trailing-separator edge case", filepath.Join(realRoot, "db") + string(filepath.Separator), realRoot, false},
		{"9. . path normalization", filepath.Join(realRoot, ".", "db", ".", "nested"), realRoot, false},

		// Symlink handling
		{"10. existing symlink to a path inside root", symlinkInsideToInside, realRoot, false},
		{"11. existing symlink to a path outside root", symlinkInsideToOutside, realRoot, true},
		{"12. existing symlinked parent with non-existent final child", filepath.Join(symlinkInsideToOutside, "new_db"), realRoot, true},
		{"13. nested symlinked parent escaping the root", filepath.Join(symlinkNestedToOutside, "new_db"), realRoot, true},
		{"13b. nested symlinked parent with .. traversal escaping the root", symlinkInsideToOutside + string(filepath.Separator) + ".." + string(filepath.Separator) + "escaped", realRoot, true},

		// Existence
		{"14. non-existent child with real ancestors", filepath.Join(realRoot, "does_not_exist"), realRoot, false},

		// Root variants
		{"15. symlinked storage root, exact", symlinkedRoot, symlinkedRoot, false},
		{"15b. symlinked storage root, valid child", filepath.Join(symlinkedRoot, "db"), symlinkedRoot, false},
		{"15c. symlinked storage root, child escaping", filepath.Join(symlinkedRoot, "..", "outside"), symlinkedRoot, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStoragePath(tt.target, tt.root)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateStoragePath(%q, %q) error = %v, wantError %v", tt.target, tt.root, err, tt.wantError)
			}
		})
	}
}
