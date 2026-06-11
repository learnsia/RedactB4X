package redactorpii

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPatternLibraryPersist(t *testing.T) {
	dir := t.TempDir()
	disk := NewDiskManager(dir)
	if err := disk.Init(); err != nil {
		t.Fatal(err)
	}
	srv := &Server{disk: disk}
	srv.loadPatternLibrary()

	p, err := srv.addSavedPattern("regex", `[0-9]{3}`, "three digits")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" {
		t.Fatal("expected id")
	}

	srv2 := &Server{disk: disk}
	srv2.loadPatternLibrary()
	if len(srv2.patternLibrary) != 1 {
		t.Fatalf("expected 1 pattern after reload, got %d", len(srv2.patternLibrary))
	}
	got := srv2.resolveLibraryPatternStrings([]string{p.ID})
	if len(got) != 1 || got[0] != CustomPatternRegexPrefix+`[0-9]{3}` {
		t.Fatalf("resolve: %#v", got)
	}

	path := filepath.Join(dir, patternLibraryFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not written: %v", err)
	}
}
