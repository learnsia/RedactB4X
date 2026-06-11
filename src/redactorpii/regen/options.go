package regen

// CombineOptions controls how selected matches are stitched into one expression.
type CombineOptions struct {
	// OnlyPatterns uses .* for gaps between selections instead of escaped literals.
	OnlyPatterns bool `json:"onlyPatterns"`
	// MatchWholeLine wraps the result with ^ and $.
	MatchWholeLine bool `json:"matchWholeLine"`
	// CaseInsensitive prefixes the pattern with (?i) for Go regexp.
	CaseInsensitive bool `json:"caseInsensitive"`
}

// DefaultCombineOptions is the default combiner behavior.
var DefaultCombineOptions = CombineOptions{
	OnlyPatterns:    false,
	MatchWholeLine:  false,
	CaseInsensitive: false,
}

// Limits caps work for suggest/preview APIs.
type Limits struct {
	MaxSampleBytes int
	MaxCandidates  int
	MaxMatches     int
	MaxPatternLen  int
}

// DefaultLimits are safe defaults for API handlers.
var DefaultLimits = Limits{
	MaxSampleBytes: 256 * 1024,
	MaxCandidates:  500,
	MaxMatches:     10_000,
	MaxPatternLen:  2048,
}
