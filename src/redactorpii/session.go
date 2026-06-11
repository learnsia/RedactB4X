package redactorpii

import (
	"sync"
)

// Session holds all state for the current session.
type Session struct {
	mu sync.Mutex

	ScenarioID string

	// Document processing
	ProcessedDocs  map[string]*RedactionResult `json:"processedDocs"`
	ApprovedDocs   map[string]bool             `json:"approvedDocs"`
	RejectedDocs   map[string]string           `json:"rejectedDocs"` // docID -> reason
	CustomPatterns []string                    `json:"customPatterns"`

	// Batch processing
	BatchSampleResults  []BatchDocResult   `json:"batchSampleResults"`
	BatchAllResults     []BatchDocResult   `json:"batchAllResults"`
	BatchProgress       BatchProgress      `json:"batchProgress"`
	BatchSampleApproved bool               `json:"batchSampleApproved"`
}

type BatchDocResult struct {
	DocID       string            `json:"docId"`
	Title       string            `json:"title"`
	Category    string            `json:"category"`
	PIIFound    int               `json:"piiFound"`
	PIIBreakdown map[string]int   `json:"piiBreakdown"`
	SampleChars int               `json:"sampleChars"`
	TotalChars  int               `json:"totalChars"`
}

type BatchProgress struct {
	Total      int    `json:"total"`
	Processed  int    `json:"processed"`
	Current    string `json:"current"`
	Running    bool   `json:"running"`
	Complete   bool   `json:"complete"`
	TotalPII   int    `json:"totalPII"`
}

func NewSession() *Session {
	return &Session{
		ProcessedDocs: make(map[string]*RedactionResult),
		ApprovedDocs:  make(map[string]bool),
		RejectedDocs:  make(map[string]string),
	}
}

func (s *Session) ProcessDoc(docID string, content string, customPatterns []string) *RedactionResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := RedactDocument(content)
	if len(customPatterns) > 0 {
		ApplyCustomPatterns(&result, customPatterns)
	}
	s.ProcessedDocs[docID] = &result
	return &result
}

func (s *Session) AddCustomPattern(pattern string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CustomPatterns = append(s.CustomPatterns, pattern)
}

func (s *Session) GetCustomPatterns() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.CustomPatterns))
	copy(cp, s.CustomPatterns)
	return cp
}

// ProcessSample processes all documents and stores results as the batch sample.
func (s *Session) ProcessSample(docs []SampleDocument) []BatchDocResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.BatchSampleResults = nil
	for _, doc := range docs {
		result := RedactDocument(doc.Content)
		if len(s.CustomPatterns) > 0 {
			ApplyCustomPatterns(&result, s.CustomPatterns)
		}
		s.ProcessedDocs[doc.ID] = &result

		breakdown := make(map[string]int)
		for _, item := range result.Items {
			breakdown[item.TypeLabel]++
		}

		s.BatchSampleResults = append(s.BatchSampleResults, BatchDocResult{
			DocID:        doc.ID,
			Title:        doc.Title,
			Category:     doc.Category,
			PIIFound:     result.TotalFound,
			PIIBreakdown: breakdown,
			SampleChars:  len(doc.Content),
			TotalChars:   len(doc.Content),
		})
	}
	s.BatchSampleApproved = false
	return s.BatchSampleResults
}

// ApproveBatchSample marks the sample as approved for full batch processing.
func (s *Session) ApproveBatchSample() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BatchSampleApproved = true
	for _, r := range s.BatchSampleResults {
		s.ApprovedDocs[r.DocID] = true
	}
}

// RunBatch processes all documents and stores full results.
func (s *Session) RunBatch(docs []SampleDocument) []BatchDocResult {
	s.mu.Lock()
	s.BatchAllResults = nil
	s.BatchProgress = BatchProgress{
		Total:   len(docs),
		Running: true,
	}
	s.mu.Unlock()

	var results []BatchDocResult
	totalPII := 0

	for i, doc := range docs {
		s.mu.Lock()
		s.BatchProgress.Processed = i
		s.BatchProgress.Current = doc.Title
		s.mu.Unlock()

		result := RedactDocument(doc.Content)
		s.mu.Lock()
		if len(s.CustomPatterns) > 0 {
			ApplyCustomPatterns(&result, s.CustomPatterns)
		}
		s.ProcessedDocs[doc.ID] = &result
		s.ApprovedDocs[doc.ID] = true
		s.mu.Unlock()

		breakdown := make(map[string]int)
		for _, item := range result.Items {
			breakdown[item.TypeLabel]++
		}
		totalPII += result.TotalFound

		results = append(results, BatchDocResult{
			DocID:        doc.ID,
			Title:        doc.Title,
			Category:     doc.Category,
			PIIFound:     result.TotalFound,
			PIIBreakdown: breakdown,
			SampleChars:  len(doc.Content),
			TotalChars:   len(doc.Content),
		})
	}

	s.mu.Lock()
	s.BatchAllResults = results
	s.BatchProgress.Processed = len(docs)
	s.BatchProgress.Running = false
	s.BatchProgress.Complete = true
	s.BatchProgress.TotalPII = totalPII
	s.mu.Unlock()

	return results
}

func (s *Session) ApproveDoc(docID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ApprovedDocs[docID] = true
	delete(s.RejectedDocs, docID)
}

func (s *Session) RejectDoc(docID string, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RejectedDocs[docID] = reason
	delete(s.ApprovedDocs, docID)
}

func (s *Session) UnprocessDoc(docID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ProcessedDocs, docID)
	delete(s.ApprovedDocs, docID)
	delete(s.RejectedDocs, docID)
}

// GetStatus returns the current state of the session for the UI.
func (s *Session) GetStatus() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := SessionStatus{
		DocumentsProcessed: len(s.ProcessedDocs),
		DocumentsApproved:  len(s.ApprovedDocs),
		DocumentsRejected:  len(s.RejectedDocs),
	}

	status.CanProceedToStep2 = status.DocumentsApproved > 0

	return status
}

type SessionStatus struct {
	DocumentsProcessed int  `json:"documentsProcessed"`
	DocumentsApproved  int  `json:"documentsApproved"`
	DocumentsRejected  int  `json:"documentsRejected"`
	CanProceedToStep2  bool `json:"canProceedToStep2"`
}
