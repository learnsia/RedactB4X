package redactorpii

import "strings"

// LibraryDocMatches returns whether a document should appear in the library list for the given normalized folder prefix and lowercased search query (empty q matches all).
func LibraryDocMatches(d SampleDocument, qLower, folderPrefixNorm string) bool {
	if !FolderPrefixMatch(d.Folder, folderPrefixNorm) {
		return false
	}
	if qLower == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		d.ID, d.Title, d.Category, d.Folder, d.SourceRelPath,
	}, " "))
	return strings.Contains(hay, qLower)
}
