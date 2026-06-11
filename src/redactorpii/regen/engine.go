package regen

import (
	"fmt"
	"regexp"
	"sort"
)

// Span is a match interval in sample text.
type Span struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

// PreviewResult contains a compiled expression and its matches in the sample.
type PreviewResult struct {
	Expression string `json:"expression"`
	Matches    []Span `json:"matches"`
	MatchCount int    `json:"matchCount"`
}

// Suggest returns recognizer candidates for sample text.
func Suggest(sample string, lim Limits) ([]Match, error) {
	return FindCandidates(sample, lim)
}

// Build composes a Go regexp string from selected matches.
func Build(sample string, selected []Match, opts CombineOptions) (BuildResult, error) {
	if lim := DefaultLimits; len(sample) > lim.MaxSampleBytes {
		return BuildResult{}, ErrSampleTooLarge
	}
	br, err := Combine(sample, selected, opts)
	if err != nil {
		return BuildResult{}, err
	}
	if len(br.Expression) > DefaultLimits.MaxPatternLen {
		return BuildResult{}, ErrPatternTooLong
	}
	if _, err := regexp.Compile(br.Expression); err != nil {
		return BuildResult{}, fmt.Errorf("%w: %v", ErrInvalidPattern, err)
	}
	return br, nil
}

// Preview builds an expression and lists all matches in the sample.
func Preview(sample string, selected []Match, opts CombineOptions, lim Limits) (PreviewResult, error) {
	if lim.MaxSampleBytes <= 0 {
		lim = DefaultLimits
	}
	br, err := Build(sample, selected, opts)
	if err != nil {
		return PreviewResult{}, err
	}
	re, err := regexp.Compile(br.Expression)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("%w: %v", ErrInvalidPattern, err)
	}
	indices := re.FindAllStringIndex(sample, -1)
	if lim.MaxMatches > 0 && len(indices) > lim.MaxMatches {
		return PreviewResult{}, ErrTooManyMatches
	}
	spans := make([]Span, 0, len(indices))
	for _, loc := range indices {
		spans = append(spans, Span{
			Start: loc[0],
			End:   loc[1],
			Text:  sliceText(sample, loc[0], loc[1]),
		})
	}
	return PreviewResult{
		Expression: br.Expression,
		Matches:    spans,
		MatchCount: len(spans),
	}, nil
}

// ResolveMatches finds matches by ID from a candidate list.
func ResolveMatches(candidates []Match, ids []string) ([]Match, error) {
	byID := make(map[string]Match, len(candidates))
	for _, m := range candidates {
		byID[m.ID] = m
	}
	var selected []Match
	for _, id := range ids {
		m, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownMatchID, id)
		}
		selected = append(selected, m)
	}
	if len(selected) == 0 && len(ids) > 0 {
		return nil, ErrUnknownMatchID
	}
	return selected, nil
}

// SelectNonOverlapping adds a match if it does not intersect existing selection.
func SelectNonOverlapping(selected []Match, candidate Match) ([]Match, bool) {
	for _, m := range selected {
		if rangesIntersect(m.Start, m.End, candidate.Start, candidate.End) {
			return selected, false
		}
	}
	return append(selected, candidate), true
}

// SortMatches sorts matches by position.
func SortMatches(matches []Match) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start != matches[j].Start {
			return matches[i].Start < matches[j].Start
		}
		return matches[i].End < matches[j].End
	})
}
