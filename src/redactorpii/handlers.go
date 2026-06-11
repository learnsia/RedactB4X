package redactorpii

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"redactb4x/internal/converter"
	"redactb4x/internal/middleware"
)

type Server struct {
	sessions  map[string]*Session
	scenarios map[string]DemoScenario
	active    string
	listenAddr string
	disk      *DiskManager
	config    *AppConfig
	docIndex  []StoredDocument
	docsDir   string
	scanRoot  string
	rateLimit *middleware.RateLimiter

	patternLibrary []SavedPattern
	patternLibMu   sync.RWMutex
}

func NewServerWithConfig(listenAddr string, cfg *AppConfig, disk *DiskManager, docsDir string) *Server {
	scanRoot, _ := os.Getwd()
	srv := &Server{
		sessions:   make(map[string]*Session),
		scenarios:  make(map[string]DemoScenario),
		active:     "custom",
		listenAddr: listenAddr,
		disk:       disk,
		config:    cfg,
		docsDir:   docsDir,
		scanRoot:  scanRoot,
	}
	if cfg != nil {
		srv.active = "custom"
	}
	docs := disk.LoadDocumentIndex()
	srv.docIndex = docs
	if cfg != nil {
		docsSample := srv.storedDocsToSample(docs)
		srv.scenarios["custom"] = CustomScenario(*cfg, docsSample)
	}

	sess := NewSession()
	if err := disk.LoadSession(sess); err != nil {
		log.Printf(`{"level":"WARN","msg":"failed to load session","error":"%s"}`, err)
	}
	srv.sessions[srv.active] = sess
	srv.loadPatternLibrary()

	return srv
}

func (s *Server) storedDocsToSample(docs []StoredDocument) []SampleDocument {
	var samples []SampleDocument
	for _, d := range docs {
		content, err := s.disk.LoadDocument(d)
		if err != nil {
			continue
		}
		samples = append(samples, SampleDocument{
			ID:            d.ID,
			Title:         d.Title,
			Category:      d.Category,
			Folder:        d.Folder,
			SourceRelPath: d.SourceRelPath,
			Content:       content,
		})
	}
	return samples
}

func (s *Server) saveSession() {
	sess := s.activeSession()
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if err := s.disk.SaveSession(sess); err != nil {
		log.Printf(`{"level":"ERROR","msg":"failed to save session","error":"%s"}`, err)
	}
}

func (s *Server) saveSessionFrom(sess *Session) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if err := s.disk.SaveSession(sess); err != nil {
		log.Printf(`{"level":"ERROR","msg":"failed to save session","error":"%s"}`, err)
	}
}

func (s *Server) saveDocIndex() {
	if err := s.disk.SaveDocumentIndex(s.docIndex); err != nil {
		log.Printf(`{"level":"ERROR","msg":"failed to save document index","error":"%s"}`, err)
	}
}

func (s *Server) activeScenario() DemoScenario {
	return s.scenarios[s.active]
}

func (s *Server) activeSession() *Session {
	if s.sessions[s.active] == nil {
		s.sessions[s.active] = NewSession()
	}
	return s.sessions[s.active]
}

// libraryDocuments returns stored uploads for the DMS library, falling back to the active scenario when none exist.
func (s *Server) libraryDocuments() []SampleDocument {
	if len(s.docIndex) > 0 {
		return s.storedDocsToSample(s.docIndex)
	}
	return s.activeScenario().Documents
}

func (s *Server) findLibraryDocument(id string) (*SampleDocument, bool) {
	for _, d := range s.libraryDocuments() {
		if d.ID == id {
			cp := d
			return &cp, true
		}
	}
	return nil, false
}

func (s *Server) syncCustomScenario() {
	if s.config != nil {
		s.scenarios["custom"] = CustomScenario(*s.config, s.storedDocsToSample(s.docIndex))
	}
}

// Handler returns the fully configured http.Handler with all routes and middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static files
	mux.HandleFunc("/", s.handleDashboard)

	// Document management
	mux.HandleFunc("/api/documents/upload", s.handleUploadDocument)
	mux.HandleFunc("/api/documents/delete", s.handleDeleteDocument)
	mux.HandleFunc("/api/documents/scan", s.handleScanDirectory)
	mux.HandleFunc("/api/documents/scan/import", s.handleScanImport)
	mux.HandleFunc("/api/documents/download", s.handleDownloadDocument)
	mux.HandleFunc("/api/documents/download-batch", s.handleDownloadBatch)

	// DMS API
	mux.HandleFunc("/api/documents", s.handleGetDocuments)
	mux.HandleFunc("/api/library/folders", s.handleLibraryFolders)
	mux.HandleFunc("/api/library/documents", s.handleLibraryDocuments)
	mux.HandleFunc("/api/documents/process", s.handleProcessDoc)
	mux.HandleFunc("/api/documents/unprocess", s.handleUnprocessDoc)
	mux.HandleFunc("/api/documents/approve", s.handleApproveDoc)
	mux.HandleFunc("/api/documents/reject", s.handleRejectDoc)
	mux.HandleFunc("/api/redact/text", s.handleRedactText)
	mux.HandleFunc("/api/documents/add-pattern", s.handleAddPattern)
	mux.HandleFunc("/api/documents/missed-names", s.handleMissedNames)
	mux.HandleFunc("/api/patterns/suggest", s.handlePatternSuggest)
	mux.HandleFunc("/api/patterns/build", s.handlePatternBuild)
	mux.HandleFunc("/api/patterns/preview", s.handlePatternPreview)
	mux.HandleFunc("/api/patterns/library", s.handlePatternLibrary)
	mux.HandleFunc("/api/batch/sample", s.handleBatchSample)
	mux.HandleFunc("/api/batch/approve", s.handleBatchApprove)
	mux.HandleFunc("/api/batch/run", s.handleBatchRun)
	mux.HandleFunc("/api/batch/progress", s.handleBatchProgress)
	mux.HandleFunc("/api/batch/results", s.handleBatchResults)
	mux.HandleFunc("/api/status", s.handleGetStatus)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/rate-limit/reset", s.handleRateLimitReset)

	s.rateLimit = middleware.NewRateLimiter(300)
	rl := s.rateLimit

	var handler http.Handler = mux
	handler = middleware.RecoverPanic(handler)
	handler = middleware.RequestLogger(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.RateLimit(rl)(handler)
	handler = middleware.MaxBodySize(32 << 20)(handler)

	return handler
}

func (s *Server) Start() error {
	server := &http.Server{
		Addr:              s.listenAddr,
		Handler:           s.Handler(),
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	log.Printf(`{"level":"INFO","msg":"redactorpii server starting","addr":"%s"}`, s.listenAddr)
	return server.ListenAndServe()
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	static := StaticHandler()
	dashboard := DashboardHTML()
	if r.URL.Path != "/" {
		static.ServeHTTP(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboard)) //nolint:go.lang.security.audit.xss.no-direct-write-to-responsewriter // constant HTML, no user data
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.activeSession().GetStatus())
}

func (s *Server) handleGetDocuments(w http.ResponseWriter, r *http.Request) {
	type docStatus struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Category     string `json:"category"`
		Folder       string `json:"folder,omitempty"`
		Processed    bool   `json:"processed"`
		Approved     bool   `json:"approved"`
		Rejected     bool   `json:"rejected"`
		PIICount     int    `json:"piiCount"`
		RedactedText string `json:"redactedText,omitempty"`
	}

	includeRedacted := r.URL.Query().Get("includeRedacted") == "1" || strings.EqualFold(r.URL.Query().Get("includeRedacted"), "true")
	var idFilter map[string]struct{}
	if raw := strings.TrimSpace(r.URL.Query().Get("docIds")); raw != "" {
		idFilter = make(map[string]struct{})
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				idFilter[part] = struct{}{}
			}
		}
	}
	if includeRedacted && len(idFilter) == 0 {
		jsonError(w, "docIds is required when includeRedacted is set", http.StatusBadRequest)
		return
	}

	s.activeSession().mu.Lock()
	defer s.activeSession().mu.Unlock()

	var docs []docStatus
	for _, d := range s.libraryDocuments() {
		if includeRedacted && len(idFilter) > 0 {
			if _, ok := idFilter[d.ID]; !ok {
				continue
			}
		}
		ds := docStatus{
			ID:       d.ID,
			Title:    d.Title,
			Category: d.Category,
			Folder:   d.Folder,
		}
		if result, ok := s.activeSession().ProcessedDocs[d.ID]; ok {
			ds.Processed = true
			ds.PIICount = result.TotalFound
			if includeRedacted {
				ds.RedactedText = result.RedactedText
			}
		}
		ds.Approved = s.activeSession().ApprovedDocs[d.ID]
		ds.Rejected = s.activeSession().RejectedDocs[d.ID] != ""
		docs = append(docs, ds)
	}
	writeJSON(w, docs)
}

func (s *Server) allLibraryFolderPaths() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range s.disk.LoadLibraryFolders() {
		n, err := NormalizeFolderPath(p)
		if err != nil || n == "" {
			continue
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	for _, d := range s.docIndex {
		if d.Folder == "" {
			continue
		}
		n, err := NormalizeFolderPath(d.Folder)
		if err != nil || n == "" {
			continue
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

func (s *Server) handleLibraryFolders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{"folders": s.allLibraryFolderPaths()})
	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		n, err := NormalizeFolderPath(req.Path)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if n == "" {
			jsonError(w, "path is required", http.StatusBadRequest)
			return
		}
		explicit := s.disk.LoadLibraryFolders()
		for _, e := range explicit {
			en, _ := NormalizeFolderPath(e)
			if en == n {
				writeJSON(w, map[string]interface{}{"status": "exists", "path": n, "folders": s.allLibraryFolderPaths()})
				return
			}
		}
		explicit = append(explicit, n)
		if err := s.disk.SaveLibraryFolders(explicit); err != nil {
			jsonError(w, "failed to save folders: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "created", "path": n, "folders": s.allLibraryFolderPaths()})
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLibraryDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type docStatus struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Category     string `json:"category"`
		Folder       string `json:"folder,omitempty"`
		Processed    bool   `json:"processed"`
		Approved     bool   `json:"approved"`
		Rejected     bool   `json:"rejected"`
		PIICount     int    `json:"piiCount"`
		RedactedText string `json:"redactedText,omitempty"`
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	folderPrefix, ferr := NormalizeFolderPath(r.URL.Query().Get("folder"))
	if ferr != nil {
		jsonError(w, ferr.Error(), http.StatusBadRequest)
		return
	}
	page := 1
	if p, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page"))); err == nil && p > 0 {
		page = p
	}
	pageSize := 20
	if ps, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("pageSize"))); err == nil && ps > 0 {
		pageSize = ps
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if pageSize < 1 {
		pageSize = 1
	}

	s.activeSession().mu.Lock()
	defer s.activeSession().mu.Unlock()

	var matched []docStatus
	for _, d := range s.libraryDocuments() {
		if !LibraryDocMatches(d, q, folderPrefix) {
			continue
		}
		ds := docStatus{
			ID:       d.ID,
			Title:    d.Title,
			Category: d.Category,
			Folder:   d.Folder,
		}
		if result, ok := s.activeSession().ProcessedDocs[d.ID]; ok {
			ds.Processed = true
			ds.PIICount = result.TotalFound
		}
		ds.Approved = s.activeSession().ApprovedDocs[d.ID]
		ds.Rejected = s.activeSession().RejectedDocs[d.ID] != ""
		matched = append(matched, ds)
	}

	total := len(matched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageItems := matched[start:end]

	writeJSON(w, map[string]interface{}{
		"items":    pageItems,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (s *Server) handleProcessDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DocID             string    `json:"docId"`
		LibraryPatternIDs *[]string `json:"libraryPatternIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	doc, ok := s.findLibraryDocument(req.DocID)
	if !ok {
		jsonError(w, "document not found", http.StatusNotFound)
		return
	}

	var patterns []string
	if req.LibraryPatternIDs != nil {
		patterns = s.resolveLibraryPatternStrings(*req.LibraryPatternIDs)
	} else {
		patterns = s.activeSession().GetCustomPatterns()
	}

	result := s.activeSession().ProcessDoc(doc.ID, doc.Content, patterns)
	s.saveSession()
	writeJSON(w, result)
}

func (s *Server) handleRedactText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text              string    `json:"text"`
		LibraryPatternIDs *[]string `json:"libraryPatternIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		jsonError(w, "text is required", http.StatusBadRequest)
		return
	}
	if len(text) > 1_000_000 {
		jsonError(w, "text too large (max 1 MB)", http.StatusBadRequest)
		return
	}

	var patterns []string
	if req.LibraryPatternIDs != nil {
		patterns = s.resolveLibraryPatternStrings(*req.LibraryPatternIDs)
	} else {
		patterns = s.activeSession().GetCustomPatterns()
	}

	result := RedactDocument(text)
	if len(patterns) > 0 {
		ApplyCustomPatterns(&result, patterns)
	}
	writeJSON(w, result)
}

func (s *Server) handleUnprocessDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DocID string `json:"docId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	s.activeSession().UnprocessDoc(req.DocID)
	s.saveSession()
	writeJSON(w, map[string]string{"status": "unprocessed", "docId": req.DocID})
}

func (s *Server) handleApproveDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DocID string `json:"docId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	s.activeSession().ApproveDoc(req.DocID)
	s.saveSession()
	writeJSON(w, map[string]string{"status": "approved", "docId": req.DocID})
}

func (s *Server) handleRejectDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DocID  string `json:"docId"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	s.activeSession().RejectDoc(req.DocID, req.Reason)
	s.saveSession()
	writeJSON(w, map[string]string{"status": "rejected", "docId": req.DocID})
}

func (s *Server) handleAddPattern(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Pattern       string `json:"pattern"`
		Expression    string `json:"expression"`
		Kind          string `json:"kind"` // "literal" (default) or "regex"
		Label         string `json:"label"`
		SaveToLibrary bool   `json:"saveToLibrary"`
		DocID         string `json:"docId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	stored := strings.TrimSpace(req.Pattern)
	if req.Kind == "regex" || req.Expression != "" {
		expr := strings.TrimSpace(req.Expression)
		if expr == "" {
			expr = stored
		}
		if expr == "" {
			jsonError(w, "expression is required for regex rules", http.StatusBadRequest)
			return
		}
		if _, err := regexp.Compile(expr); err != nil {
			jsonError(w, "invalid regex: "+err.Error(), http.StatusBadRequest)
			return
		}
		stored = CustomPatternRegexPrefix + expr
	}
	if stored == "" {
		jsonError(w, "pattern is required", http.StatusBadRequest)
		return
	}

	s.activeSession().AddCustomPattern(stored)

	var saved *SavedPattern
	if req.SaveToLibrary {
		kind := req.Kind
		if kind == "" {
			if strings.HasPrefix(stored, CustomPatternRegexPrefix) {
				kind = "regex"
			} else {
				kind = "literal"
			}
		}
		expr := strings.TrimPrefix(stored, CustomPatternRegexPrefix)
		if kind == "literal" {
			expr = stored
		}
		p, err := s.addSavedPattern(kind, expr, req.Label)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.Pattern = p.displayExpression()
		saved = &p
	}

	// Re-process the document with the new pattern
	doc, ok := s.findLibraryDocument(req.DocID)
	if !ok {
		jsonError(w, "document not found", http.StatusNotFound)
		return
	}

	patterns := s.activeSession().GetCustomPatterns()
	result := s.activeSession().ProcessDoc(doc.ID, doc.Content, patterns)
	s.saveSession()
	resp := map[string]interface{}{"result": result}
	if saved != nil {
		resp["savedPattern"] = saved
	}
	writeJSON(w, resp)
}

func (s *Server) handleMissedNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DocID string `json:"docId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	doc, ok := s.findLibraryDocument(req.DocID)
	if !ok {
		jsonError(w, "document not found", http.StatusNotFound)
		return
	}

	s.activeSession().mu.Lock()
	result := s.activeSession().ProcessedDocs[req.DocID]
	s.activeSession().mu.Unlock()

	if result == nil {
		result = s.activeSession().ProcessDoc(doc.ID, doc.Content, s.activeSession().GetCustomPatterns())
	}

	missed := GetMissedNames(doc.Content, result)
	writeJSON(w, map[string]interface{}{
		"missedNames": missed,
		"patterns":    s.activeSession().GetCustomPatterns(),
	})
}

func (s *Server) handleBatchSample(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	results := s.activeSession().ProcessSample(s.libraryDocuments())
	writeJSON(w, map[string]interface{}{
		"results":        results,
		"totalDocs":      len(results),
		"customPatterns": s.activeSession().GetCustomPatterns(),
	})
}

func (s *Server) handleBatchApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.activeSession().ApproveBatchSample()
	s.saveSession()
	writeJSON(w, map[string]string{"status": "approved"})
}

func (s *Server) handleBatchRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	results := s.activeSession().RunBatch(s.libraryDocuments())
	s.saveSession()
	writeJSON(w, map[string]interface{}{
		"results":   results,
		"totalDocs": len(results),
		"totalPII":  s.activeSession().BatchProgress.TotalPII,
	})
}

func (s *Server) handleBatchProgress(w http.ResponseWriter, r *http.Request) {
	s.activeSession().mu.Lock()
	defer s.activeSession().mu.Unlock()
	writeJSON(w, s.activeSession().BatchProgress)
}

func (s *Server) handleBatchResults(w http.ResponseWriter, r *http.Request) {
	s.activeSession().mu.Lock()
	defer s.activeSession().mu.Unlock()
	writeJSON(w, map[string]interface{}{
		"sample":   s.activeSession().BatchSampleResults,
		"all":      s.activeSession().BatchAllResults,
		"progress": s.activeSession().BatchProgress,
		"patterns": s.activeSession().CustomPatterns,
	})
}

// ═══════════ DOCUMENT UPLOAD & MANAGEMENT ═══════════

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Support both multipart form and JSON body
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		s.handleMultipartUpload(w, r)
	} else {
		s.handleJSONUpload(w, r)
	}
}

func (s *Server) handleMultipartUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20) // 32MB max

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "no file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	category := r.FormValue("category")
	if category == "" {
		category = "general"
	}
	folder := r.FormValue("folder")
	folderNorm, ferr := NormalizeFolderPath(folder)
	if ferr != nil {
		jsonError(w, ferr.Error(), http.StatusBadRequest)
		return
	}
	folder = folderNorm

	ext := strings.ToLower(filepath.Ext(header.Filename))
	content, err := io.ReadAll(file)
	if err != nil {
		jsonError(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	storeName := header.Filename
	storeBody := string(content)

	if converter.NeedsConversion(ext) {
		md, convErr := converter.ConvertToMarkdown(header.Filename, content)
		if convErr != nil {
			jsonError(w, "document conversion failed: "+convErr.Error(), http.StatusBadRequest)
			return
		}
		base := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
		storeName = base + ".md"
		storeBody = md
	} else if ext != ".txt" && ext != ".md" && ext != ".csv" && ext != ".json" {
		jsonError(w, "unsupported file type. Allowed: .txt, .md, .csv, .json, or convertible formats (.pdf, .docx, .xlsx, .pptx, .html, .epub, .ipynb)", http.StatusBadRequest)
		return
	}

	doc, err := s.disk.StoreDocumentWithMeta(StoreDocumentInput{
		LogicalFilename: storeName,
		Folder:          folder,
		Category:        category,
		Content:         storeBody,
	})
	if err != nil {
		jsonError(w, "failed to store document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.docIndex = append(s.docIndex, *doc)
	s.saveDocIndex()

	s.syncCustomScenario()

	writeJSON(w, map[string]interface{}{
		"status":   "uploaded",
		"document": doc,
		"total":    len(s.docIndex),
	})
}

func (s *Server) handleJSONUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename string `json:"filename"`
		Category string `json:"category"`
		Folder   string `json:"folder"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Filename == "" || req.Content == "" {
		jsonError(w, "filename and content are required", http.StatusBadRequest)
		return
	}
	if req.Category == "" {
		req.Category = "general"
	}
	folderNorm, ferr := NormalizeFolderPath(req.Folder)
	if ferr != nil {
		jsonError(w, ferr.Error(), http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(req.Filename))
	if converter.NeedsConversion(ext) {
		jsonError(w, "JSON upload does not support binary office files; use multipart form upload for PDF, Word, Excel, PowerPoint, HTML, EPUB, or Jupyter", http.StatusBadRequest)
		return
	}
	if ext != ".txt" && ext != ".md" && ext != ".csv" && ext != ".json" {
		jsonError(w, "unsupported file type for JSON upload", http.StatusBadRequest)
		return
	}

	doc, err := s.disk.StoreDocumentWithMeta(StoreDocumentInput{
		LogicalFilename: req.Filename,
		Folder:          folderNorm,
		Category:        req.Category,
		Content:         req.Content,
	})
	if err != nil {
		jsonError(w, "failed to store document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.docIndex = append(s.docIndex, *doc)
	s.saveDocIndex()

	s.syncCustomScenario()

	writeJSON(w, map[string]interface{}{
		"status":   "uploaded",
		"document": doc,
		"total":    len(s.docIndex),
	})
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		DocID string `json:"docId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	var found bool
	var newDocs []StoredDocument
	for _, d := range s.docIndex {
		if d.ID == req.DocID {
			s.disk.DeleteDocument(d)
			found = true
		} else {
			newDocs = append(newDocs, d)
		}
	}
	if !found {
		jsonError(w, "document not found", http.StatusNotFound)
		return
	}

	s.docIndex = newDocs
	s.saveDocIndex()

	s.syncCustomScenario()

	writeJSON(w, map[string]interface{}{
		"status": "deleted",
		"docId":  req.DocID,
		"total":  len(s.docIndex),
	})
}

func (s *Server) handleDownloadDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docID := strings.TrimSpace(r.URL.Query().Get("id"))
	if docID == "" {
		jsonError(w, "id query parameter required", http.StatusBadRequest)
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "original"
	}

	var found *StoredDocument
	for i := range s.docIndex {
		if s.docIndex[i].ID == docID {
			found = &s.docIndex[i]
			break
		}
	}
	if found == nil {
		jsonError(w, "document not found", http.StatusNotFound)
		return
	}

	var data string
	filename := found.Filename

	if format == "redacted" {
		sess := s.activeSession()
		sess.mu.Lock()
		result, ok := sess.ProcessedDocs[docID]
		sess.mu.Unlock()
		if !ok || result == nil {
			jsonError(w, "document not yet processed — process it first", http.StatusBadRequest)
			return
		}
		data = result.RedactedText
		base := sanitizeFilename(strings.TrimSuffix(filename, filepath.Ext(filename)))
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+base+"-redacted.md\"")
	} else {
		raw, err := s.disk.LoadDocument(*found)
		if err != nil {
			jsonError(w, "failed to read document: "+err.Error(), http.StatusInternalServerError)
			return
		}
		data = raw
		if format == "markdown" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			if strings.HasSuffix(strings.ToLower(filename), ".md") {
				w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(filename)+"\"")
			} else {
				base := sanitizeFilename(strings.TrimSuffix(filename, filepath.Ext(filename)))
				w.Header().Set("Content-Disposition", "attachment; filename=\""+base+".md\"")
			}
		} else {
			ext := strings.ToLower(filepath.Ext(filename))
			contentType := "application/octet-stream"
			switch ext {
			case ".txt", ".md":
				contentType = "text/plain; charset=utf-8"
			case ".csv":
				contentType = "text/csv; charset=utf-8"
			case ".json":
				contentType = "application/json; charset=utf-8"
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(filename)+"\"")
		}
	}

	w.Write([]byte(data)) //nolint:go.lang.security.audit.xss.no-direct-write-to-responsewriter // content-type is non-HTML (markdown/plain/csv/json/zip)
}

func (s *Server) handleDownloadBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DocIDs []string `json:"docIds"`
		Format string   `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.DocIDs) == 0 {
		jsonError(w, "docIds required", http.StatusBadRequest)
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "original"
	}
	if format != "original" && format != "markdown" && format != "redacted" {
		jsonError(w, "format must be original, markdown, or redacted", http.StatusBadRequest)
		return
	}

	type docEntry struct {
		stored  *StoredDocument
		content string
	}
	var entries []docEntry

	sess := s.activeSession()
	for _, docID := range req.DocIDs {
		docID = strings.TrimSpace(docID)
		var found *StoredDocument
		for i := range s.docIndex {
			if s.docIndex[i].ID == docID {
				found = &s.docIndex[i]
				break
			}
		}
		if found == nil {
			continue
		}

		var content string
		if format == "redacted" {
			sess.mu.Lock()
			result, ok := sess.ProcessedDocs[docID]
			sess.mu.Unlock()
			if !ok || result == nil {
				continue
			}
			content = result.RedactedText
		} else {
			raw, err := s.disk.LoadDocument(*found)
			if err != nil {
				continue
			}
			content = raw
		}
		entries = append(entries, docEntry{stored: found, content: content})
	}

	if len(entries) == 0 {
		jsonError(w, "no valid documents found", http.StatusNotFound)
		return
	}

	suffix := ""
	if format == "markdown" {
		suffix = ".md"
	} else if format == "redacted" {
		suffix = "-redacted.md"
	}

	if len(entries) == 1 {
		e := entries[0]
		filename := e.stored.Filename
		if suffix != "" {
			base := strings.TrimSuffix(filename, filepath.Ext(filename))
			filename = base + suffix
		}
		contentType := "text/plain; charset=utf-8"
		if suffix == ".md" || suffix == "-redacted.md" {
			contentType = "text/markdown; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(filename)+"\"")
		w.Write([]byte(e.content)) //nolint:go.lang.security.audit.xss.no-direct-write-to-responsewriter // content-type is non-HTML (markdown/plain)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		base := strings.TrimSuffix(e.stored.Filename, filepath.Ext(e.stored.Filename))
		if base == "" {
			base = "document"
		}
		fName := base + suffix
		f, err := zw.Create(fName)
		if err != nil {
			jsonError(w, "zip error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := f.Write([]byte(e.content)); err != nil {
			jsonError(w, "zip write error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := zw.Close(); err != nil {
		jsonError(w, "zip close error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"documents.zip\"")
	w.Write(buf.Bytes()) //nolint:go.lang.security.audit.xss.no-direct-write-to-responsewriter // binary zip content
}

func (s *Server) handleScanDirectory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{
			"defaultDir": s.docsDir,
		})
		return
	case http.MethodPost:
		// continue below
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := s.docsDir
	var req struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && strings.TrimSpace(req.Dir) != "" {
		dir = req.Dir
	}

	if strings.TrimSpace(dir) == "" {
		jsonError(w, "no directory specified. Enter a path in the scan dialog or start the server with -docs-dir", http.StatusBadRequest)
		return
	}

	absDir, resolveErr := ResolveScanDirectory(dir)
	if resolveErr != nil {
		jsonError(w, resolveErr.Error(), http.StatusBadRequest)
		return
	}
	if !isWithinScanRoot(s.scanRoot, absDir) {
		jsonError(w, "directory is outside the allowed scan root", http.StatusForbidden)
		return
	}

	scanResult, err := ScanDirectory(absDir)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	added, skipped := s.ingestScannedDocuments(scanResult.Documents)
	s.writeScanResponse(w, dir, scanResult, added, skipped)
}

func (s *Server) handleScanImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Documents []SampleDocument `json:"documents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Documents) == 0 {
		jsonError(w, "no documents provided", http.StatusBadRequest)
		return
	}
	if len(req.Documents) > 500 {
		jsonError(w, "too many documents in one import (max 500)", http.StatusBadRequest)
		return
	}

	var docs []SampleDocument
	for _, doc := range req.Documents {
		src := filepath.ToSlash(strings.TrimSpace(doc.SourceRelPath))
		if src == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(src))
		if !IsScannableExt(ext) {
			continue
		}
		dirPart := filepath.ToSlash(filepath.Dir(src))
		folder := ""
		if dirPart != "." {
			folder = dirPart
		}
		docs = append(docs, SampleDocument{
			Title:         strings.TrimSuffix(filepath.Base(src), ext),
			Category:      FirstPathSegment(folder),
			Folder:        folder,
			SourceRelPath: src,
			Content:       doc.Content,
		})
	}
	if len(docs) == 0 {
		jsonError(w, "no supported documents (.txt, .md, .csv, .json) in import", http.StatusBadRequest)
		return
	}

	added, skipped := s.ingestScannedDocuments(docs)
	s.writeScanResponse(w, "browser folder", DirectoryScanResult{Documents: docs}, added, skipped)
}

func (s *Server) ingestScannedDocuments(docs []SampleDocument) (added, skipped int) {
	existingSource := make(map[string]struct{})
	for _, existing := range s.docIndex {
		if existing.SourceRelPath != "" {
			existingSource[filepath.ToSlash(existing.SourceRelPath)] = struct{}{}
		}
	}
	for _, doc := range docs {
		src := filepath.ToSlash(strings.TrimSpace(doc.SourceRelPath))
		if src != "" {
			if _, ok := existingSource[src]; ok {
				skipped++
				continue
			}
		}
		ext := strings.ToLower(filepath.Ext(doc.SourceRelPath))
		if ext == "" {
			ext = ".txt"
		}
		logicalName := doc.Title + ext
		stored, err := s.disk.StoreDocumentWithMeta(StoreDocumentInput{
			LogicalFilename: logicalName,
			Folder:          doc.Folder,
			Category:        doc.Category,
			Content:         doc.Content,
			SourceRelPath:   src,
		})
		if err != nil {
			continue
		}
		s.docIndex = append(s.docIndex, *stored)
		if src != "" {
			existingSource[src] = struct{}{}
		}
		added++
	}

	s.saveDocIndex()

	s.syncCustomScenario()
	return added, skipped
}

func (s *Server) writeScanResponse(w http.ResponseWriter, dir string, scanResult DirectoryScanResult, added, skipped int) {
	writeJSON(w, map[string]interface{}{
		"status":      "scanned",
		"directory":   dir,
		"found":       len(scanResult.Documents),
		"added":       added,
		"skipped":     skipped,
		"warnings":    scanResult.Warnings,
		"totalStored": len(s.docIndex),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleRateLimitReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Reject requests from non-loopback addresses.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || !ip.IsLoopback() {
		jsonError(w, "rate-limit reset is restricted to localhost", http.StatusForbidden)
		return
	}
	if s.rateLimit != nil {
		s.rateLimit.Reset("")
	}
	writeJSON(w, map[string]string{
		"status":  "ok",
		"message": "rate limits cleared",
	})
}

// sanitizeFilename strips control characters (CR, LF, NUL) and quotes from a
// filename so it is safe to embed in a Content-Disposition header.
func sanitizeFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch r {
		case '\r', '\n', '\x00', '"':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isWithinScanRoot checks that target is within the allowed scan root directory.
func isWithinScanRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	// filepath.Rel returns a path starting with ".." when target is outside root.
	return !strings.HasPrefix(rel, "..")
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
