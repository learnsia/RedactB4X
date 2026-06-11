package regen

import (
	"regexp"
	"testing"
)

func TestRegistryPatternsCompile(t *testing.T) {
	for _, r := range Recognizers() {
		if _, err := r.FindMatches("test 123"); err != nil {
			t.Errorf("recognizer %q FindMatches: %v", r.Name(), err)
		}
	}
	if len(Recognizers()) < 10 {
		t.Fatalf("expected at least 10 recognizers, got %d", len(Recognizers()))
	}
}

func TestCombineOnlyPatternsSingleSelection(t *testing.T) {
	sample := "abc123def"
	selected := []Match{{
		ID: "Number:3:6", Start: 3, End: 6, Pattern: `[0-9]+`, Recognizer: "Number",
	}}
	br, err := Combine(sample, selected, CombineOptions{OnlyPatterns: true})
	if err != nil {
		t.Fatal(err)
	}
	if br.Expression != `[0-9]+` {
		t.Fatalf("expression = %q, want [0-9]+", br.Expression)
	}
}

func TestCombineLiteralGaps(t *testing.T) {
	sample := "Routing 021000089"
	selected := []Match{{
		ID: "n:8:17", Start: 8, End: 17, Pattern: `[0-9]+`, Recognizer: "Number",
	}}
	br, err := Combine(sample, selected, CombineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := `Routing [0-9]+`
	if br.Expression != want {
		t.Fatalf("expression = %q, want %q", br.Expression, want)
	}
}

func TestPreviewFindsMatches(t *testing.T) {
	sample := "id 100 and id 200"
	selected := []Match{{
		ID: "n:3:6", Start: 3, End: 6, Pattern: `[0-9]+`, Recognizer: "Number",
	}}
	prev, err := Preview(sample, selected, CombineOptions{OnlyPatterns: true}, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if prev.MatchCount < 2 {
		t.Fatalf("match count = %d, want >= 2", prev.MatchCount)
	}
}

func TestBuildCompilesInGo(t *testing.T) {
	sample := "EMP-20481"
	cands, err := Suggest(sample, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	var digit Match
	for _, c := range cands {
		if c.Recognizer == "Number" && c.Start == 4 {
			digit = c
			break
		}
	}
	if digit.ID == "" {
		t.Fatal("expected Number match at 4")
	}
	br, err := Build(sample, []Match{digit}, CombineOptions{OnlyPatterns: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := regexp.Compile(br.Expression); err != nil {
		t.Fatalf("compile: %v expr=%q", err, br.Expression)
	}
}

func TestPersonNameThreeWords(t *testing.T) {
	sample := "421-90-3440 f 1953/07/17 Kroeger Morrison Adriane\n" +
		"451-80-3526 m 1950/06/09 Parmer Santos Thomas\n" +
		"300-62-3266 m 1965/02/10 Spain Faulkner Victo\n" +
		"Dickerson Oyola Lynette"
	cands, err := FindCandidates(sample, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	var names []Match
	for _, c := range cands {
		if c.Recognizer == "Person name (3+ words)" {
			names = append(names, c)
		}
	}
	if len(names) < 4 {
		t.Fatalf("expected at least 4 person-name matches, got %d: %v", len(names), names)
	}
	br, err := Build(sample, []Match{names[len(names)-1]}, CombineOptions{OnlyPatterns: true})
	if err != nil {
		t.Fatal(err)
	}
	prev, err := Preview(sample, []Match{names[len(names)-1]}, CombineOptions{OnlyPatterns: true}, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if prev.MatchCount < 4 {
		t.Fatalf("preview match count = %d, want >= 4, expr=%q", prev.MatchCount, br.Expression)
	}
}

func TestOverlappingSelectionRejected(t *testing.T) {
	_, err := Combine("abc", []Match{
		{ID: "a:0:1", Start: 0, End: 1, Pattern: "a"},
		{ID: "b:0:2", Start: 0, End: 2, Pattern: "ab"},
	}, CombineOptions{})
	if err != ErrOverlappingMatch {
		t.Fatalf("err = %v, want ErrOverlappingMatch", err)
	}
}

func TestResolveMatches(t *testing.T) {
	cands := []Match{{ID: "x:1:2", Start: 1, End: 2, Pattern: "b"}}
	got, err := ResolveMatches(cands, []string{"x:1:2"})
	if err != nil || len(got) != 1 {
		t.Fatalf("ResolveMatches: %v %v", got, err)
	}
	_, err = ResolveMatches(cands, []string{"missing"})
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}
