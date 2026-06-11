package redactorpii

import (
	"path/filepath"
	"testing"
)

func TestSaveLibraryFolders_dedupAndSort(t *testing.T) {
	d := NewDiskManager(t.TempDir())
	if err := d.Init(); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveLibraryFolders([]string{"b/a", "a", "a", ""}); err != nil {
		t.Fatal(err)
	}
	got := d.LoadLibraryFolders()
	if len(got) != 2 || got[0] != "a" || got[1] != "b/a" {
		t.Fatalf("got %#v", got)
	}
}

func TestStoreDocumentWithMeta_nestedFolder(t *testing.T) {
	d := NewDiskManager(t.TempDir())
	if err := d.Init(); err != nil {
		t.Fatal(err)
	}
	doc, err := d.StoreDocumentWithMeta(StoreDocumentInput{
		LogicalFilename: "note.md",
		Folder:          "reg/sub",
		Category:        "general",
		Content:         "hello",
		SourceRelPath:   "reg/sub/note.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Folder != "reg/sub" {
		t.Errorf("folder %q", doc.Folder)
	}
	rel := filepath.Join("data", "documents", filepath.FromSlash("reg/sub"), doc.ID+".md")
	wantPath := filepath.Join(d.base, rel)
	if doc.StoredPath != wantPath {
		t.Errorf("path\n got %s\n want %s", doc.StoredPath, wantPath)
	}
	body, err := d.LoadDocument(*doc)
	if err != nil || body != "hello" {
		t.Fatalf("read err=%v body=%q", err, body)
	}
}
