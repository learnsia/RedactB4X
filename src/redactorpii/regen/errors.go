package regen

import "errors"

var (
	ErrSampleTooLarge    = errors.New("sample text exceeds size limit")
	ErrPatternTooLong    = errors.New("generated pattern exceeds size limit")
	ErrInvalidPattern    = errors.New("pattern does not compile in Go regexp")
	ErrTooManyMatches    = errors.New("too many matches in preview")
	ErrUnknownMatchID    = errors.New("unknown match id")
	ErrOverlappingMatch  = errors.New("selected matches overlap")
	ErrNoSelection       = errors.New("no matches selected")
)
