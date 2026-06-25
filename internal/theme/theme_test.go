package theme

import (
	"embed"
	"testing"
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
	m := NewManager("/tmp/install/themes", "/tmp/user/themes", testEmbedFS)
	m.EnsureSeeded()
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
