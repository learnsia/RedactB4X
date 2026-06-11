package redactorpii

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeFolderPath(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"a/b/c", "a/b/c", false},
		{"/a/b/", "a/b", false},
		{"a/../b", "", true},
		{"..", "", true},
	}
	for _, tt := range tests {
		got, err := NormalizeFolderPath(tt.in)
		if (err != nil) != tt.wantErr {
			t.Fatalf("NormalizeFolderPath(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("NormalizeFolderPath(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestFolderPrefixMatch(t *testing.T) {
	if !FolderPrefixMatch("regulatory/pci", "") {
		t.Error("empty prefix should match")
	}
	if !FolderPrefixMatch("regulatory/pci", "regulatory") {
		t.Error("nested should match prefix")
	}
	if FolderPrefixMatch("other", "regulatory") {
		t.Error("unrelated should not match")
	}
	if !FolderPrefixMatch("regulatory", "regulatory") {
		t.Error("exact match")
	}
}

func TestFirstPathSegment(t *testing.T) {
	if got := FirstPathSegment(""); got != "general" {
		t.Errorf("empty: %q", got)
	}
	if got := FirstPathSegment("only"); got != "only" {
		t.Errorf("single: %q", got)
	}
	if got := FirstPathSegment("a/b"); got != "a" {
		t.Errorf("nested: %q", got)
	}
}

func TestLibraryDocMatches(t *testing.T) {
	d := SampleDocument{
		ID:            "abc123",
		Title:         "Handbook",
		Category:      "HR",
		Folder:        "policies/hr",
		SourceRelPath: "policies/hr/handbook.md",
	}
	if !LibraryDocMatches(d, "", "") {
		t.Fatal("no filters")
	}
	if !LibraryDocMatches(d, "hand", "policies") {
		t.Fatal("q + folder prefix")
	}
	if LibraryDocMatches(d, "", "finance") {
		t.Fatal("wrong folder")
	}
	if LibraryDocMatches(d, "missingterm", "") {
		t.Fatal("q mismatch")
	}
}

func TestScanDirectory_nestedFolder(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "org", "policies")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "rule.md"), []byte("# x"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ScanDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	docs := result.Documents
	if len(docs) != 1 {
		t.Fatalf("len=%d", len(docs))
	}
	if docs[0].Folder != "org/policies" {
		t.Errorf("Folder=%q", docs[0].Folder)
	}
	if docs[0].Title != "rule" {
		t.Errorf("Title=%q", docs[0].Title)
	}
	if docs[0].SourceRelPath != filepath.ToSlash(filepath.Join("org", "policies", "rule.md")) {
		t.Errorf("SourceRelPath=%q", docs[0].SourceRelPath)
	}
}

func TestScanDirectory_permissionWarningContinues(t *testing.T) {
	root := t.TempDir()
	pub := filepath.Join(root, "open")
	priv := filepath.Join(root, "closed")
	if err := os.MkdirAll(pub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(priv, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(priv, 0755)
	if err := os.WriteFile(filepath.Join(pub, "visible.md"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("len=%d", len(result.Documents))
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for inaccessible directory")
	}
}
