package converter

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	markitdown "github.com/conductor-oss/markitdown"
)

var convertibleExts = map[string]struct{}{
	".pdf":   {},
	".docx":  {},
	".xlsx":  {},
	".xls":   {},
	".pptx":  {},
	".html":  {},
	".htm":   {},
	".epub":  {},
	".ipynb": {},
	".csv":   {},
	".rss":   {},
	".atom":  {},
	".xml":   {},
	".zip":   {},
}

func NeedsConversion(ext string) bool {
	_, ok := convertibleExts[strings.ToLower(ext)]
	return ok
}

func ConvertToMarkdown(filename string, raw []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == ".txt" || ext == ".md" || ext == ".json" || ext == ".jsonl" {
		return string(raw), nil
	}

	if !NeedsConversion(ext) {
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}

	m := markitdown.New()
	result, err := m.ConvertReader(bytes.NewReader(raw), markitdown.StreamInfo{
		Extension: ext,
		MIMEType:  mimeForExt(ext),
	})
	if err != nil {
		return "", fmt.Errorf("markitdown: %w", err)
	}
	return result.Markdown, nil
}

func mimeForExt(ext string) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".html", ".htm":
		return "text/html"
	case ".epub":
		return "application/epub+zip"
	case ".ipynb":
		return "application/x-ipynb+json"
	case ".csv":
		return "text/csv"
	case ".xml":
		return "application/xml"
	default:
		return ""
	}
}
