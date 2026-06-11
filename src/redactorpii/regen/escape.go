package regen

import "regexp"

func escapeForRegex(s string) string {
	return regexp.QuoteMeta(s)
}
