package regen

import (
	"fmt"
	"regexp"
	"strings"
)

// SimpleRecognizer finds spans using a search regex and emits a composable output pattern.
type SimpleRecognizer struct {
	label          string
	OutputPattern  string
	SearchPattern  string // optional; %s is replaced with OutputPattern
	MainGroupIndex int
}

func (s *SimpleRecognizer) Name() string { return s.label }

func NewSimpleRecognizer(name, outputPattern string) *SimpleRecognizer {
	return &SimpleRecognizer{
		label:          name,
		OutputPattern:  outputPattern,
		MainGroupIndex: 1,
	}
}

func (s *SimpleRecognizer) withSearch(search string) *SimpleRecognizer {
	cp := *s
	cp.SearchPattern = search
	return &cp
}

func (s *SimpleRecognizer) searchRegex() (*regexp.Regexp, error) {
	search := s.SearchPattern
	if search == "" {
		search = "(" + s.OutputPattern + ")"
	} else if strings.Contains(search, "%s") {
		search = fmt.Sprintf(search, s.OutputPattern)
	}
	return regexp.Compile(search)
}

func (s *SimpleRecognizer) FindMatches(input string) ([]Match, error) {
	re, err := s.searchRegex()
	if err != nil {
		return nil, err
	}
	indices := re.FindAllStringSubmatchIndex(input, -1)
	gi := s.MainGroupIndex
	var out []Match
	for _, loc := range indices {
		if len(loc) < (gi+1)*2 {
			continue
		}
		start, end := loc[gi*2], loc[gi*2+1]
		if start < 0 || end < start {
			continue
		}
		out = append(out, Match{
			ID:         matchID(s.label, start, end),
			Title:      s.label,
			Start:      start,
			End:        end,
			Text:       sliceText(input, start, end),
			Pattern:    s.OutputPattern,
			Recognizer: s.label,
		})
	}
	return out, nil
}
