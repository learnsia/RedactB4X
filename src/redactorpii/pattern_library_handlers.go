package redactorpii

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handlePatternLibrary(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		patterns := s.listSavedPatterns()
		for i := range patterns {
			patterns[i].Pattern = patterns[i].displayExpression()
		}
		writeJSON(w, map[string]interface{}{"patterns": patterns})
	case http.MethodPost:
		s.handlePatternLibraryCreate(w, r)
	case http.MethodDelete:
		s.handlePatternLibraryDelete(w, r)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePatternLibraryCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label     string `json:"label"`
		Kind      string `json:"kind"`
		Pattern   string `json:"pattern"`
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	pattern := strings.TrimSpace(req.Pattern)
	if req.Expression != "" {
		pattern = strings.TrimSpace(req.Expression)
	}
	kind := req.Kind
	if kind == "" && strings.HasPrefix(pattern, CustomPatternRegexPrefix) {
		kind = "regex"
		pattern = strings.TrimPrefix(pattern, CustomPatternRegexPrefix)
	}
	if kind == "" {
		kind = "literal"
	}
	p, err := s.addSavedPattern(kind, pattern, req.Label)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	out := p
	out.Pattern = out.displayExpression()
	writeJSON(w, map[string]interface{}{"pattern": out})
}

func (s *Server) handlePatternLibraryDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = strings.TrimSpace(req.ID)
		}
	}
	if id == "" {
		jsonError(w, "id is required", http.StatusBadRequest)
		return
	}
	if err := s.deleteSavedPattern(id); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted", "id": id})
}
