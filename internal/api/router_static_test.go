package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeStaticFilePath(t *testing.T) {
	root := t.TempDir()
	assetDir := filepath.Join(root, "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetFile := filepath.Join(assetDir, "app.js")
	if err := os.WriteFile(assetFile, []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatal(err)
	}

	assertUnderRoot := func(t *testing.T, resolved string) {
		t.Helper()
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("path %q escapes static root %q", resolved, root)
		}
	}

	tests := []struct {
		name    string
		urlPath string
		want    string
		ok      bool
	}{
		{name: "asset file", urlPath: "/assets/app.js", want: assetFile, ok: true},
		{name: "missing file under root", urlPath: "/assets/missing.js", ok: true},
		{name: "root path", urlPath: "/", ok: false},
		{name: "absolute traversal attempt", urlPath: "/../../../etc/passwd", ok: true},
		{name: "nested traversal attempt", urlPath: "/assets/../../etc/passwd", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := safeStaticFilePath(root, tt.urlPath)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (path %q)", ok, tt.ok, got)
			}
			if !ok {
				return
			}
			assertUnderRoot(t, got)
			if tt.want != "" && got != tt.want {
				t.Fatalf("path = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("legacy join escapes root", func(t *testing.T) {
		legacy := filepath.Join(root, "/../../../etc/passwd")
		rel, err := filepath.Rel(root, legacy)
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			t.Skip("platform did not reproduce join escape")
		}

		got, ok := safeStaticFilePath(root, "/../../../etc/passwd")
		if !ok {
			t.Fatal("expected safe resolution")
		}
		assertUnderRoot(t, got)
	})
}
