package regen

import (
	"fmt"
	"regexp"
	"strings"
)

// EchoRecognizer matches with an inner regex and emits escaped literals or a template.
type EchoRecognizer struct {
	label           string
	InnerPattern    string
	OutputTemplate  string // optional; %1$s replaced with escaped match text
	inner           *regexp.Regexp
}

func (e *EchoRecognizer) Name() string { return e.label }

func NewEchoRecognizer(name, innerPattern string) *EchoRecognizer {
	return &EchoRecognizer{label: name, InnerPattern: innerPattern}
}

func (e *EchoRecognizer) WithOutputTemplate(tmpl string) *EchoRecognizer {
	cp := *e
	cp.OutputTemplate = tmpl
	return &cp
}

func (e *EchoRecognizer) compile() error {
	if e.inner != nil {
		return nil
	}
	re, err := regexp.Compile(e.InnerPattern)
	if err != nil {
		return err
	}
	e.inner = re
	return nil
}

func (e *EchoRecognizer) FindMatches(input string) ([]Match, error) {
	if err := e.compile(); err != nil {
		return nil, err
	}
	var out []Match
	for _, loc := range e.inner.FindAllStringIndex(input, -1) {
		start, end := loc[0], loc[1]
		val := sliceText(input, start, end)
		pat := escapeForRegex(val)
		if e.OutputTemplate != "" {
			pat = strings.ReplaceAll(e.OutputTemplate, "%1$s", escapeForRegex(val))
		}
		title := e.label
		if val != "" {
			title = fmt.Sprintf("%s (%s)", e.label, val)
		}
		out = append(out, Match{
			ID:         matchID(e.label, start, end),
			Title:      title,
			Start:      start,
			End:        end,
			Text:       val,
			Pattern:    pat,
			Recognizer: e.label,
		})
	}
	return out, nil
}
