package regen

import "fmt"

// Match is a candidate span in sample text with a composable pattern fragment.
type Match struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Start      int    `json:"start"`
	End        int    `json:"end"` // exclusive
	Text       string `json:"text"`
	Pattern    string `json:"pattern"`
	Recognizer string `json:"recognizer"`
}

// Recognizer finds candidate matches in sample text.
type Recognizer interface {
	Name() string
	FindMatches(input string) ([]Match, error)
}

func matchID(recognizer string, start, end int) string {
	return fmt.Sprintf("%s:%d:%d", recognizer, start, end)
}

func sliceText(input string, start, end int) string {
	if start < 0 || end > len(input) || start >= end {
		return ""
	}
	return input[start:end]
}
