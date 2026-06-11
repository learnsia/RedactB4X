package redactorpii

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	dataDir       = "data"
	stateDir      = "data/state"
	documentsDir  = "data/documents"
	redactedDir   = "data/state/redacted"
	reportsDir    = "data/reports"
	configFile    = "data/config.json"
	sessionFile   = "data/state/session.json"
	docIndexFile       = "data/state/documents.json"
	libraryFoldersFile = "data/state/library-folders.json"
	patternLibraryFile = "data/state/pattern-library.json"
)

// AppConfig holds user-defined configuration for the production system.
type AppConfig struct {
	Company             string `json:"company"`
	Industry            string `json:"industry"`
	ComplianceFramework string `json:"complianceFramework"`
	ComplianceVersion   string `json:"complianceVersion"`
	Description         string `json:"description"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

// StoredDocument tracks an uploaded document.
type StoredDocument struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	Title         string `json:"title"`
	Category      string `json:"category"`
	Folder        string `json:"folder,omitempty"`
	SourceRelPath string `json:"sourceRelPath,omitempty"`
	StoredPath    string `json:"storedPath"`
	Size          int64  `json:"size"`
	UploadedAt    string `json:"uploadedAt"`
}

// StoreDocumentInput describes content to persist under data/documents with optional folder layout.
type StoreDocumentInput struct {
	LogicalFilename string // basename preferred; extension sets stored file suffix
	Folder          string
	Category        string
	Content         string
	SourceRelPath string // optional: scan-relative path for deduplication
}

// DiskManager handles all persistence operations.
type DiskManager struct {
	mu   sync.Mutex
	base string
}

// NewDiskManager creates a disk manager rooted at the given base directory.
func NewDiskManager(base string) *DiskManager {
	return &DiskManager{base: base}
}

// Init creates all required directories.
func (d *DiskManager) Init() error {
	dirs := []string{
		filepath.Join(d.base, "data"),
		filepath.Join(d.base, "data/state"),
		filepath.Join(d.base, "data/state/redacted"),
		filepath.Join(d.base, "data/documents"),
		filepath.Join(d.base, "data/reports"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return nil
}

// SaveConfig persists the application configuration.
func (d *DiskManager) SaveConfig(cfg AppConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if cfg.CreatedAt == "" {
		cfg.CreatedAt = cfg.UpdatedAt
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.base, configFile), data, 0644)
}

// LoadConfig loads the application configuration. Returns nil if not found.
func (d *DiskManager) LoadConfig() *AppConfig {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(d.base, configFile))
	if err != nil {
		return nil
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// SaveSession persists the session state.
func (d *DiskManager) SaveSession(sess *Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	snapshot := sessionSnapshot{
		ProcessedDocs:     sess.ProcessedDocs,
		ApprovedDocs:      sess.ApprovedDocs,
		RejectedDocs:      sess.RejectedDocs,
		CustomPatterns:    sess.CustomPatterns,
		SavedAt:           time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.base, sessionFile), data, 0644)
}

// LoadSession loads persisted session state into an existing session.
func (d *DiskManager) LoadSession(sess *Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(d.base, sessionFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var snapshot sessionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if snapshot.ProcessedDocs != nil {
		sess.ProcessedDocs = snapshot.ProcessedDocs
	}
	if snapshot.ApprovedDocs != nil {
		sess.ApprovedDocs = snapshot.ApprovedDocs
	}
	if snapshot.RejectedDocs != nil {
		sess.RejectedDocs = snapshot.RejectedDocs
	}
	if snapshot.CustomPatterns != nil {
		sess.CustomPatterns = snapshot.CustomPatterns
	}

	return nil
}

type sessionSnapshot struct {
	ProcessedDocs  map[string]*RedactionResult `json:"processedDocs"`
	ApprovedDocs   map[string]bool             `json:"approvedDocs"`
	RejectedDocs   map[string]string           `json:"rejectedDocs"`
	CustomPatterns []string                    `json:"customPatterns"`
	SavedAt        string                      `json:"savedAt"`
}

// SaveDocumentIndex persists the document index.
func (d *DiskManager) SaveDocumentIndex(docs []StoredDocument) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.base, docIndexFile), data, 0644)
}

// LoadDocumentIndex loads the persisted document index.
func (d *DiskManager) LoadDocumentIndex() []StoredDocument {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(d.base, docIndexFile))
	if err != nil {
		return nil
	}
	var docs []StoredDocument
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil
	}
	return docs
}

func newDocID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// StoreDocumentWithMeta writes content under data/documents/[folder/]<id><ext> and returns metadata.
func (d *DiskManager) StoreDocumentWithMeta(in StoreDocumentInput) (*StoredDocument, error) {
	folderNorm, err := NormalizeFolderPath(in.Folder)
	if err != nil {
		return nil, err
	}
	logical := strings.TrimSpace(in.LogicalFilename)
	if logical == "" {
		return nil, fmt.Errorf("filename is required")
	}
	ext := filepath.Ext(logical)
	baseTitle := strings.TrimSuffix(filepath.Base(logical), ext)
	if baseTitle == "" {
		baseTitle = "document"
	}
	id, err := newDocID()
	if err != nil {
		return nil, err
	}
	onDisk := id + ext
	var storePath string
	if folderNorm == "" {
		storePath = filepath.Join(d.base, documentsDir, onDisk)
	} else {
		storePath = filepath.Join(d.base, documentsDir, filepath.FromSlash(folderNorm), onDisk)
	}
	parentDir := filepath.Dir(storePath)

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("creating parent dirs: %w", err)
	}
	if err := os.WriteFile(storePath, []byte(in.Content), 0644); err != nil {
		return nil, fmt.Errorf("storing document: %w", err)
	}

	info, _ := os.Stat(storePath)
	src := in.SourceRelPath
	if src != "" {
		src = filepath.ToSlash(strings.TrimSpace(src))
	}
	doc := &StoredDocument{
		ID:            id,
		Filename:      filepath.Base(logical),
		Title:         baseTitle,
		Category:      in.Category,
		Folder:        folderNorm,
		SourceRelPath: src,
		StoredPath:    storePath,
		Size:          info.Size(),
		UploadedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	return doc, nil
}

// StoreDocument saves an uploaded document at the library root (no folder) with an opaque id.
func (d *DiskManager) StoreDocument(filename, category, content string) (*StoredDocument, error) {
	base := filepath.Base(strings.TrimSpace(filename))
	if base == "" || base == "." {
		base = "document.txt"
	}
	return d.StoreDocumentWithMeta(StoreDocumentInput{
		LogicalFilename: base,
		Folder:          "",
		Category:        category,
		Content:         content,
	})
}

// LoadDocument reads a stored document's content from disk.
func (d *DiskManager) LoadDocument(doc StoredDocument) (string, error) {
	data, err := os.ReadFile(doc.StoredPath)
	if err != nil {
		return "", fmt.Errorf("reading document %s: %w", doc.Filename, err)
	}
	return string(data), nil
}

// DeleteDocument removes a stored document from disk.
func (d *DiskManager) DeleteDocument(doc StoredDocument) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return os.Remove(doc.StoredPath)
}

// DirectoryScanResult holds documents discovered during a directory scan and non-fatal warnings.
type DirectoryScanResult struct {
	Documents []SampleDocument
	Warnings  []string
}

// ScanDirectory scans a directory for documents and returns them as SampleDocuments.
// Folder is the full relative directory under dir (forward slashes); SourceRelPath is the relative file path.
// Permission and read errors are recorded in Warnings; the scan continues for accessible files.
func ScanDirectory(dir string) (DirectoryScanResult, error) {
	absDir, err := ResolveScanDirectory(dir)
	if err != nil {
		return DirectoryScanResult{}, err
	}

	var result DirectoryScanResult
	walkErr := filepath.Walk(absDir, func(path string, info os.FileInfo, visitErr error) error {
		if visitErr != nil {
			rel := path
			if r, relErr := filepath.Rel(absDir, path); relErr == nil {
				rel = filepath.ToSlash(r)
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipped %s: %v", rel, visitErr))
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !IsScannableExt(ext) {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			relPath, relErr := filepath.Rel(absDir, path)
			if relErr != nil {
				relPath = path
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("cannot read %s: %v", filepath.ToSlash(relPath), readErr))
			return nil
		}
		relPath, relErr := filepath.Rel(absDir, path)
		if relErr != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		dirPart := filepath.ToSlash(filepath.Dir(relPath))
		folder := ""
		if dirPart != "." {
			folder = dirPart
		}
		category := FirstPathSegment(folder)
		result.Documents = append(result.Documents, SampleDocument{
			ID:            relPath,
			Title:         strings.TrimSuffix(info.Name(), ext),
			Category:      category,
			Folder:        folder,
			SourceRelPath: relPath,
			Content:       string(content),
		})
		return nil
	})
	if walkErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("scan interrupted: %v", walkErr))
	}
	return result, nil
}

// LoadLibraryFolders returns persisted user-created folder paths (normalized).
func (d *DiskManager) LoadLibraryFolders() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(d.base, libraryFoldersFile))
	if err != nil {
		return nil
	}
	var folders []string
	if err := json.Unmarshal(data, &folders); err != nil {
		return nil
	}
	return folders
}

// SaveLibraryFolders persists user-created folder paths (deduplicated, sorted).
func (d *DiskManager) SaveLibraryFolders(folders []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	seen := make(map[string]struct{})
	var out []string
	for _, f := range folders {
		n, err := NormalizeFolderPath(f)
		if err != nil || n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	slices.Sort(out)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(d.base, stateDir), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.base, libraryFoldersFile), data, 0644)
}

// LoadPatternLibrary reads persisted custom pattern rules.
func (d *DiskManager) LoadPatternLibrary() ([]SavedPattern, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(d.base, patternLibraryFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var file PatternLibraryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return file.Patterns, nil
}

// SavePatternLibrary writes all custom pattern rules.
func (d *DiskManager) SavePatternLibrary(patterns []SavedPattern) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := os.MkdirAll(filepath.Join(d.base, stateDir), 0755); err != nil {
		return err
	}
	file := PatternLibraryFile{Patterns: patterns}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d.base, patternLibraryFile), data, 0644)
}
