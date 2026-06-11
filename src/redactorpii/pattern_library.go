package redactorpii

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// SavedPattern is a named custom redaction rule persisted across sessions.
type SavedPattern struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"` // "regex" or "literal"
	Pattern   string `json:"pattern"`
	CreatedAt string `json:"createdAt"`
}

// PatternLibraryFile is the on-disk shape for data/state/pattern-library.json.
type PatternLibraryFile struct {
	Patterns []SavedPattern `json:"patterns"`
}

func (p SavedPattern) storageValue() string {
	p.Pattern = strings.TrimSpace(p.Pattern)
	if p.Kind == "regex" {
		if strings.HasPrefix(p.Pattern, CustomPatternRegexPrefix) {
			return p.Pattern
		}
		return CustomPatternRegexPrefix + p.Pattern
	}
	return p.Pattern
}

func (p SavedPattern) displayExpression() string {
	if p.Kind == "regex" {
		return strings.TrimPrefix(p.Pattern, CustomPatternRegexPrefix)
	}
	return p.Pattern
}

func validateSavedPattern(kind, pattern, label string) (SavedPattern, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "literal"
	}
	if kind != "literal" && kind != "regex" {
		return SavedPattern{}, fmt.Errorf("kind must be literal or regex")
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return SavedPattern{}, fmt.Errorf("pattern is required")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		if kind == "regex" {
			label = truncateLabel(pattern, 48)
		} else {
			label = truncateLabel(pattern, 40)
		}
	}
	if kind == "regex" {
		expr := strings.TrimPrefix(pattern, CustomPatternRegexPrefix)
		if err := checkRegexComplexity(expr); err != nil {
			return SavedPattern{}, err
		}
		if _, err := regexp.Compile(expr); err != nil {
			return SavedPattern{}, fmt.Errorf("invalid regex: %w", err)
		}
		pattern = expr
	}
	return SavedPattern{
		Kind:      kind,
		Pattern:   pattern,
		Label:     label,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func truncateLabel(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func (s *Server) loadPatternLibrary() {
	lib, err := s.disk.LoadPatternLibrary()
	if err != nil {
		logPatternLibraryError("load", err)
		return
	}
	s.patternLibMu.Lock()
	s.patternLibrary = lib
	s.patternLibMu.Unlock()
}

func logPatternLibraryError(op string, err error) {
	if err != nil {
		log.Printf(`{"level":"WARN","msg":"pattern library %s","error":"%s"}`, op, err)
	}
}

func (s *Server) resolveLibraryPatternStrings(ids []string) []string {
	s.patternLibMu.RLock()
	defer s.patternLibMu.RUnlock()
	byID := make(map[string]SavedPattern, len(s.patternLibrary))
	for _, p := range s.patternLibrary {
		byID[p.ID] = p
	}
	var out []string
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, p.storageValue())
		}
	}
	return out
}

func (s *Server) findSavedPattern(id string) (SavedPattern, bool) {
	s.patternLibMu.RLock()
	defer s.patternLibMu.RUnlock()
	for _, p := range s.patternLibrary {
		if p.ID == id {
			return p, true
		}
	}
	return SavedPattern{}, false
}

func (s *Server) listSavedPatterns() []SavedPattern {
	s.patternLibMu.RLock()
	defer s.patternLibMu.RUnlock()
	out := make([]SavedPattern, len(s.patternLibrary))
	copy(out, s.patternLibrary)
	return out
}

func (s *Server) persistPatternLibrary() error {
	s.patternLibMu.RLock()
	snap := make([]SavedPattern, len(s.patternLibrary))
	copy(snap, s.patternLibrary)
	s.patternLibMu.RUnlock()
	return s.disk.SavePatternLibrary(snap)
}

func (s *Server) addSavedPattern(kind, pattern, label string) (SavedPattern, error) {
	p, err := validateSavedPattern(kind, pattern, label)
	if err != nil {
		return SavedPattern{}, err
	}
	id, err := newDocID()
	if err != nil {
		return SavedPattern{}, err
	}
	p.ID = id
	s.patternLibMu.Lock()
	s.patternLibrary = append(s.patternLibrary, p)
	s.patternLibMu.Unlock()
	if err := s.persistPatternLibrary(); err != nil {
		return SavedPattern{}, err
	}
	return p, nil
}

func (s *Server) deleteSavedPattern(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	s.patternLibMu.Lock()
	var next []SavedPattern
	var found bool
	for _, p := range s.patternLibrary {
		if p.ID == id {
			found = true
			continue
		}
		next = append(next, p)
	}
	if !found {
		s.patternLibMu.Unlock()
		return fmt.Errorf("pattern not found")
	}
	s.patternLibrary = next
	s.patternLibMu.Unlock()
	return s.persistPatternLibrary()
}

// checkRegexComplexity rejects regex patterns longer than 256 characters or
// containing nested quantifiers such as (a+)+, (a*)*, or (a?)*.
func checkRegexComplexity(expr string) error {
	const maxLen = 256
	if len(expr) > maxLen {
		return fmt.Errorf("regex too long (max %d characters)", maxLen)
	}
	// Detect nested quantifiers: a group followed by a quantifier where the group
	// itself contains a quantifier on the same character class or literal.
	// Patterns: (X+)+, (X*)+, (X+)*, (X*)*, (X?)+, (X?)*, (X+){n,}, (X*){n,}
	nestedQuantifiers := regexp.MustCompile(`\([^)]*[+*][^)]*\)[+*{]`)
	if nestedQuantifiers.MatchString(expr) {
		return fmt.Errorf("regex contains nested quantifiers which may cause catastrophic backtracking; simplify the pattern")
	}
	return nil
}
