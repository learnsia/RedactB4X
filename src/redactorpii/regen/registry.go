package regen

import (
	"sort"
	"sync"
)

var (
	registryOnce sync.Once
	registryList []Recognizer
)

func defaultRecognizers() []Recognizer {
	return []Recognizer{
		NewEchoRecognizer("Character", ".").WithOutputTemplate("%1$s"),
		NewEchoRecognizer("Exact number", `[0-9]{2,}`),
		NewEchoRecognizer("Repeating character", `(.)\1{2,}`).WithOutputTemplate(`%1$s+`),
		NewSimpleRecognizer("One whitespace", `\s`),
		NewSimpleRecognizer("Whitespaces", `\s+`),
		NewSimpleRecognizer("One arbitrary character", `[a-zA-Z]`),
		NewSimpleRecognizer("Multiple characters", `[a-zA-Z]+`),
		// Three or more Title-case words (e.g. "Kroeger Morrison Adriane", "Dickerson Oyola Lynette")
		NewSimpleRecognizer("Person name (3+ words)", `\b[A-Z][A-Za-z]{1,39}(?:\s+[A-Z][A-Za-z]{1,39}){2}\b`),
		NewSimpleRecognizer("SSN", `\b\d{3}-\d{2}-\d{4}\b`),
		NewSimpleRecognizer("Date YYYY/MM/DD", `\b(?:19|20)\d{2}/\d{2}/\d{2}\b`),
		NewSimpleRecognizer("Digit", `\d`),
		NewSimpleRecognizer("Number", `[0-9]+`),
		NewSimpleRecognizer("Decimal number", `[0-9]*\.[0-9]+`),
		NewSimpleRecognizer("Alphanumeric characters", `[A-Za-z0-9]+`),
		NewSimpleRecognizer("URL encoded character", `%[0-9A-Fa-f][0-9A-Fa-f]`),
		NewSimpleRecognizer("Day", `(0?[1-9]|[12][0-9]|3[01])`).withSearch(`(?:^|\D)(%s)($|\D)`),
		NewSimpleRecognizer("Month", `(0?[1-9]|1[0-2])`).withSearch(`(?:^|\D)(%s)($|\D)`),
		NewSimpleRecognizer("Hour", `(0?[0-9]|1[0-9]|2[0-3])`).withSearch(`(?:^|\D)(%s)($|\D)`),
		NewSimpleRecognizer("Minute/ Second", `(0?[0-9]|[1-5][0-9])`).withSearch(`(?:^|\D)(%s)($|\D)`),
		NewSimpleRecognizer("Date", `[0-9]{4}-[0-9]{2}-[0-9]{2}`),
		NewSimpleRecognizer("Time", `[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,3})?`),
		NewSimpleRecognizer("Email", `[-a-z0-9!#$%&'*+/=?^_`+"`"+`{|}~]+(?:\.[-a-z0-9!#$%&'*+/=?^_`+"`"+`{|}~]+)*@(?:[a-z0-9](?:[-a-z0-9]*[a-z0-9])?\.)+[a-z0-9](?:[-a-z0-9]*[a-z0-9])?`),
		NewSimpleRecognizer("IPv4 address", `\b(?:(?:2(?:[0-4][0-9]|5[0-5])|[0-1]?[0-9]?[0-9])\.){3}(?:(?:2([0-4][0-9]|5[0-5])|[0-1]?[0-9]?[0-9]))\b`),
		NewSimpleRecognizer("UUID", `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`).withSearch(`\b(%s)\b`),
		NewSimpleRecognizer("US ZIP code", `\d{5}(?:-\d{4})?`),
		NewSimpleRecognizer("US phone number", `[+]?(?:\(\d+(?:\.\d+)?\)|\d+(?:\.\d+)?)(?:[ -]?(?:\(\d+(?:\.\d+)?\)|\d+(?:\.\d+)?))*(?:[ ]?(?:x|ext)\.?[ ]?\d{1,5})?`),
	}
}

func loadRegistry() {
	var valid []Recognizer
	for _, r := range defaultRecognizers() {
		switch x := r.(type) {
		case *SimpleRecognizer:
			if _, err := x.searchRegex(); err == nil {
				valid = append(valid, r)
			}
		case *EchoRecognizer:
			if err := x.compile(); err == nil {
				valid = append(valid, r)
			}
		}
	}
	registryList = valid
}

func Recognizers() []Recognizer {
	registryOnce.Do(loadRegistry)
	return registryList
}

// FindCandidates runs all registry recognizers on sample text.
func FindCandidates(sample string, lim Limits) ([]Match, error) {
	if lim.MaxSampleBytes <= 0 {
		lim = DefaultLimits
	}
	if len(sample) > lim.MaxSampleBytes {
		return nil, ErrSampleTooLarge
	}
	var all []Match
	for _, r := range Recognizers() {
		matches, err := r.FindMatches(sample)
		if err != nil {
			continue
		}
		all = append(all, matches...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Start != all[j].Start {
			return all[i].Start < all[j].Start
		}
		return all[i].End < all[j].End
	})
	if lim.MaxCandidates > 0 && len(all) > lim.MaxCandidates {
		all = all[:lim.MaxCandidates]
	}
	return all, nil
}
