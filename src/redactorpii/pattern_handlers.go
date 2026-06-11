package redactorpii

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"redactb4x/redactorpii/regen"
)

func (s *Server) handlePatternSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SampleText string `json:"sampleText"`
		DocID      string `json:"docId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	sample, err := s.patternSampleText(req.SampleText, req.DocID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	candidates, err := regen.Suggest(sample, regen.DefaultLimits)
	if err != nil {
		patternJSONError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{
		"candidates":  candidates,
		"sampleText":  sample,
		"sampleLen":   len(sample),
	})
}

func (s *Server) handlePatternBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handlePatternBuildPreview(w, r, false)
}

func (s *Server) handlePatternPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handlePatternBuildPreview(w, r, true)
}

func (s *Server) handlePatternBuildPreview(w http.ResponseWriter, r *http.Request, withPreview bool) {
	var req struct {
		SampleText  string             `json:"sampleText"`
		DocID       string             `json:"docId"`
		SelectedIDs []string           `json:"selectedIds"`
		Options     regen.CombineOptions `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	sample, err := s.patternSampleText(req.SampleText, req.DocID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	candidates, err := regen.Suggest(sample, regen.DefaultLimits)
	if err != nil {
		patternJSONError(w, err)
		return
	}
	selected, err := regen.ResolveMatches(candidates, req.SelectedIDs)
	if err != nil {
		patternJSONError(w, err)
		return
	}
	if withPreview {
		prev, err := regen.Preview(sample, selected, req.Options, regen.DefaultLimits)
		if err != nil {
			patternJSONError(w, err)
			return
		}
		writeJSON(w, prev)
		return
	}
	br, err := regen.Build(sample, selected, req.Options)
	if err != nil {
		patternJSONError(w, err)
		return
	}
	writeJSON(w, br)
}

func (s *Server) patternSampleText(sampleText, docID string) (string, error) {
	sampleText = strings.TrimSpace(sampleText)
	if sampleText != "" {
		if len(sampleText) > regen.DefaultLimits.MaxSampleBytes {
			return "", errors.New("sample text too large")
		}
		return sampleText, nil
	}
	if docID == "" {
		return "", errors.New("sampleText or docId is required")
	}
	doc, ok := s.findLibraryDocument(docID)
	if !ok {
		return "", errors.New("document not found")
	}
	if len(doc.Content) > regen.DefaultLimits.MaxSampleBytes {
		return "", errors.New("document too large for pattern lab")
	}
	return doc.Content, nil
}

func patternJSONError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, regen.ErrSampleTooLarge):
		jsonError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, regen.ErrPatternTooLong),
		errors.Is(err, regen.ErrInvalidPattern),
		errors.Is(err, regen.ErrOverlappingMatch),
		errors.Is(err, regen.ErrUnknownMatchID):
		jsonError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, regen.ErrTooManyMatches):
		jsonError(w, err.Error(), http.StatusUnprocessableEntity)
	default:
		jsonError(w, err.Error(), http.StatusBadRequest)
	}
}
