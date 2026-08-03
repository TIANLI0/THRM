package theme

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

//go:embed testdata
var testEmbedFS embed.FS

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/install/themes", "/tmp/user/themes", testEmbedFS)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManager_NilFS(t *testing.T) {
	m := NewManager("/tmp/install/themes", "/tmp/user/themes", nil)
	if m == nil {
		t.Fatal("NewManager with nil FS returned nil")
	}
}

func TestList_EmptyFS(t *testing.T) {
	m := NewManager("/tmp/install/themes", "/tmp/user/themes", testEmbedFS)
	metas := m.List()
	if len(metas) > 0 {
		t.Logf("List returned %d entries from empty testdata", len(metas))
	}
}

func TestReadCSS_NotFound(t *testing.T) {
	m := NewManager("/tmp/install/themes", "/tmp/user/themes", testEmbedFS)
	_, err := m.ReadCSS("nonexistent")
	if err == nil {
		t.Error("ReadCSS should fail for nonexistent theme")
	}
}

func TestResolveDir(t *testing.T) {
	m := NewManager("/tmp/install/themes", "/tmp/user/themes", testEmbedFS)
	dir := m.ResolveDir()
	if dir == "" {
		t.Error("ResolveDir should not be empty")
	}
}

func TestSourceConstants(t *testing.T) {
	if SourceUser == "" || SourceInstall == "" || SourceBuiltin == "" {
		t.Error("Source constants should not be empty")
	}
}

func TestEnsureSeeded(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install", "themes")
	userDir := filepath.Join(root, "user", "themes")
	builtin := fstest.MapFS{
		"sample/theme.json": &fstest.MapFile{Data: []byte(`{"id":"sample","name":"Sample","base":"dark"}`)},
		"sample/theme.css":  &fstest.MapFile{Data: []byte(":root {}")},
	}
	m := NewManager(installDir, userDir, builtin)
	m.EnsureSeeded()

	if _, err := os.Stat(filepath.Join(userDir, "sample", manifestName)); err != nil {
		t.Fatalf("builtin theme was not seeded into the user directory: %v", err)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("theme seeding wrote to the installation directory: %v", err)
	}
}

func TestMetaFields(t *testing.T) {
	meta := Meta{
		ID:   "test",
		Name: "Test Theme",
	}
	if meta.ID == "" {
		t.Error("Meta.ID should not be empty after assignment")
	}
}
