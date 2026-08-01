package domain

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateArchitectureExportContainsExpectedFiles(t *testing.T) {
	t.Parallel()

	inv := DemoArchitectureInventory()
	bundle := GenerateArchitectureExport(inv, nil)
	paths := map[string]bool{}
	for _, f := range bundle.Files {
		paths[f.Path] = true
		assert.NotEmpty(t, f.Content, f.Path)
	}
	want := []string{
		"MANIFEST.md",
		"c4/context.puml",
		"c4/containers.mmd",
		"diagrams/architecture.mmd",
		"diagrams/architecture.puml",
		"diagrams/architecture.excalidraw",
		"diagrams/architecture.drawio",
		"docs/architecture.md",
		"adr/0001-jetstream-topology.md",
		"adr/0002-subject-boundaries.md",
	}
	for _, p := range want {
		assert.True(t, paths[p], "missing %s", p)
	}
}

func TestZipArchitectureExportRoundTrip(t *testing.T) {
	t.Parallel()

	bundle := GenerateArchitectureExport(DemoArchitectureInventory(), nil)
	raw, err := ZipArchitectureExport(bundle)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(zr.File), 10)
}

func TestBuildArchitectureInventoryTruncates(t *testing.T) {
	t.Parallel()

	streams := make([]EventArchitectureInput, 0, archExportMaxStreams+5)
	for i := range archExportMaxStreams + 5 {
		streams = append(streams, EventArchitectureInput{Name: string(rune('A'+(i%26))) + string(rune('0'+i%10))})
	}
	// ensure unique names
	for i := range streams {
		streams[i].Name = "S" + string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	inv := BuildArchitectureInventory("c", time.Now().UTC(), streams)
	assert.True(t, inv.Truncated)
	assert.LessOrEqual(t, len(inv.Streams), archExportMaxStreams)
}

func TestADROverride(t *testing.T) {
	t.Parallel()
	bundle := GenerateArchitectureExport(DemoArchitectureInventory(), map[string]string{
		"adr/0001-jetstream-topology.md": "# polished\n",
	})
	found := false
	for _, f := range bundle.Files {
		if f.Path == "adr/0001-jetstream-topology.md" {
			assert.Equal(t, "# polished\n", f.Content)
			found = true
		}
	}
	assert.True(t, found)
}
