package regen

import (
	"sort"
	"strings"
)

type regexPart struct {
	start, end int // inclusive end for gap ranges in input
	pattern    string
	original   string // non-empty when gap is literal escape
	title      string
}

// BuildResult is a composed Go-compatible regular expression.
type BuildResult struct {
	Expression string   `json:"expression"`
	Parts      []string `json:"parts,omitempty"`
}

// Combine stitches selected matches into one regular expression.
func Combine(sample string, selected []Match, opts CombineOptions) (BuildResult, error) {
	selected = dedupeSelected(selected)
	if err := checkNonOverlapping(selected); err != nil {
		return BuildResult{}, err
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Start < selected[j].Start
	})

	var parts []regexPart
	if len(selected) == 0 {
		if opts.OnlyPatterns {
			parts = []regexPart{{pattern: ".*", title: "anything"}}
		} else {
			parts = []regexPart{{pattern: escapeForRegex(sample), original: sample}}
		}
		addWholeLineParts(&parts, opts)
		return finalize(parts, opts)
	}

	var ranged []regexPart
	for _, m := range selected {
		ranged = append(ranged, regexPart{
			start:   m.Start,
			end:     m.End - 1,
			pattern: m.Pattern,
			title:   m.Title,
		})
	}

	hasBefore := ranged[0].start > 0
	hasAfter := ranged[len(ranged)-1].end < len(sample)-1

	var first, last *regexPart
	if hasBefore {
		if opts.OnlyPatterns {
			first = &regexPart{start: 0, end: ranged[0].start - 1, pattern: ".*", title: "anything"}
		} else {
			text := sample[0:ranged[0].start]
			first = &regexPart{start: 0, end: ranged[0].start - 1, pattern: escapeForRegex(text), original: text}
		}
	}
	if hasAfter {
		if opts.OnlyPatterns {
			last = &regexPart{
				start:   ranged[len(ranged)-1].end + 1,
				end:     len(sample) - 1,
				pattern: ".*",
				title:   "anything",
			}
		} else {
			text := sample[ranged[len(ranged)-1].end+1:]
			last = &regexPart{
				start:    ranged[len(ranged)-1].end + 1,
				end:      len(sample) - 1,
				pattern:  escapeForRegex(text),
				original: text,
			}
		}
	}

	combined := combineParts(sample, first, ranged, last, opts)
	addWholeLineParts(&combined, opts)
	return finalize(combined, opts)
}

func combineParts(sample string, first *regexPart, middle []regexPart, last *regexPart, opts CombineOptions) []regexPart {
	var parts []regexPart
	if opts.MatchWholeLine || (first != nil && first.original != "") {
		if first != nil {
			parts = append(parts, *first)
		}
	}
	for i, p := range middle {
		if i > 0 {
			if gap := gapPart(sample, middle[i-1], p, opts); gap != nil {
				parts = append(parts, *gap)
			}
		}
		parts = append(parts, p)
	}
	if opts.MatchWholeLine || (last != nil && last.original != "") {
		if last != nil {
			parts = append(parts, *last)
		}
	}
	return parts
}

func gapPart(sample string, a, b regexPart, opts CombineOptions) *regexPart {
	start := a.end + 1
	end := b.start - 1
	if start > end {
		return nil
	}
	if opts.OnlyPatterns {
		return &regexPart{start: start, end: end, pattern: ".*"}
	}
	text := sample[start : end+1]
	return &regexPart{start: start, end: end, pattern: escapeForRegex(text), original: text}
}

func addWholeLineParts(parts *[]regexPart, opts CombineOptions) {
	if !opts.MatchWholeLine {
		return
	}
	*parts = append([]regexPart{{pattern: "^", title: "Start of input"}}, *parts...)
	*parts = append(*parts, regexPart{pattern: "$", title: "End of input"})
}

func finalize(parts []regexPart, opts CombineOptions) (BuildResult, error) {
	var strParts []string
	var debug []string
	for _, p := range parts {
		strParts = append(strParts, p.pattern)
		if p.title != "" {
			debug = append(debug, p.title+": "+p.pattern)
		} else {
			debug = append(debug, p.pattern)
		}
	}
	expr := strings.Join(strParts, "")
	if opts.CaseInsensitive && expr != "" && !strings.HasPrefix(expr, "(?i)") {
		expr = "(?i)" + expr
	}
	return BuildResult{Expression: expr, Parts: debug}, nil
}

func dedupeSelected(selected []Match) []Match {
	seen := make(map[string]bool)
	var out []Match
	for _, m := range selected {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	return out
}

func checkNonOverlapping(selected []Match) error {
	if len(selected) < 2 {
		return nil
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Start < selected[j].Start })
	for i := 1; i < len(selected); i++ {
		if selected[i].Start < selected[i-1].End {
			return ErrOverlappingMatch
		}
	}
	return nil
}

func rangesIntersect(aStart, aEnd, bStart, bEnd int) bool {
	return aStart < bEnd && bStart < aEnd
}
