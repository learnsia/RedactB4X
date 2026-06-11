package redactorpii

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxFolderPathLen = 512

// NormalizeFolderPath returns a canonical folder path: forward slashes, no leading/trailing slash, empty for root.
func NormalizeFolderPath(folder string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "", nil
	}
	folder = filepath.ToSlash(folder)
	folder = strings.Trim(folder, "/")
	if folder == "" {
		return "", nil
	}
	if strings.Contains(folder, "..") {
		return "", fmt.Errorf("invalid folder path")
	}
	parts := strings.Split(folder, "/")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return "", fmt.Errorf("invalid folder path")
		}
		out = append(out, p)
	}
	res := strings.Join(out, "/")
	if len(res) > maxFolderPathLen {
		return "", fmt.Errorf("folder path too long")
	}
	return res, nil
}

// FolderPrefixMatch reports whether docFolder is exactly prefix or a nested path under prefix.
// prefix "" matches all. Both sides are normalized with forward slashes.
func FolderPrefixMatch(docFolder, prefix string) bool {
	docFolder = filepath.ToSlash(strings.TrimSpace(docFolder))
	prefix = filepath.ToSlash(strings.TrimSpace(prefix))
	if prefix == "" {
		return true
	}
	prefix = strings.Trim(prefix, "/")
	if docFolder == prefix {
		return true
	}
	if docFolder == "" {
		return false
	}
	return strings.HasPrefix(docFolder, prefix+"/")
}

// ResolveScanDirectory validates and returns the absolute path of a directory to scan.
func ResolveScanDirectory(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("directory path is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("invalid directory path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

// IsScannableExt reports whether ext (including dot) is a plain-text document extension.
func IsScannableExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".txt", ".md", ".csv", ".json":
		return true
	default:
		return false
	}
}

// FirstPathSegment returns the first segment of a slash-separated path, or "general" if empty.
func FirstPathSegment(folder string) string {
	folder = filepath.ToSlash(strings.Trim(folder, "/"))
	if folder == "" {
		return "general"
	}
	if i := strings.IndexByte(folder, '/'); i >= 0 {
		return folder[:i]
	}
	return folder
}
