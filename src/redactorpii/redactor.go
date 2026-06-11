package redactorpii

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ═══════════ TOKEN-LEVEL PATTERNS (structured PII) ═══════════

var (
	reEmail       = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	rePhone       = regexp.MustCompile(`\(\d{3}\)\s*\d{3}[-.\s]?\d{4}`)
	reSSN         = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	reCreditCard  = regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`)
	reIP          = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	reRouting     = regexp.MustCompile(`Routing\s+\d{9}`)
	reAccount     = regexp.MustCompile(`Account\s+(?:Number:\s*)?\d{6,}`)
	reTaxID       = regexp.MustCompile(`Tax\s+ID:\s*\d{2}-\d{7}`)
	reInsuranceID = regexp.MustCompile(`(?:Policy|Member|Subscriber)\*{0,2}\s*(?:ID|Number|#)?\*{0,2}[:\s*:]+[A-Z]{0,3}-?\d{6,12}`)
	reNPI         = regexp.MustCompile(`NPI\*{0,2}[:\s*:]+\d{10}`)
	reMRN         = regexp.MustCompile(`(?:MRN|Medical Record(?: Number)?)\*{0,2}[:\s*:]+\d{6,10}`)
)

// ═══════════ LINE-LEVEL PATTERNS (contextual PII) ═══════════

type LinePattern struct {
	Re   *regexp.Regexp
	Type string
}

// Markdown-aware: match **Label:** value or Label: value, capture value only
var linePatterns = []LinePattern{
	// Person names
	{regexp.MustCompile(`(?i)\*{0,2}(?:Patient|Client|Student|Subscriber|Donor)?\s*Name:\*{0,2}\s*(.+)`), "PERSON"},
	{regexp.MustCompile(`(?i)\*{0,2}Emergency Contact:\*{0,2}\s*(.+)`), "CONTACT"},
	{regexp.MustCompile(`(?i)\*{0,2}(?:Treating|Referring|Primary)\s+(?:Therapist|Physician|Doctor|Provider):\*{0,2}\s*(.+)`), "PROVIDER"},
	{regexp.MustCompile(`(?i)\*{0,2}(?:Parent|Guardian)(?:/(?:Parent|Guardian))?\s*(?:Name)?:\*{0,2}\s*(.+)`), "PERSON"},
	{regexp.MustCompile(`(?i)\*{0,2}Witness:\*{0,2}\s*(.+)`), "PERSON"},
	{regexp.MustCompile(`(?i)\*{0,2}(?:Approved|Signed)\s+by:\*{0,2}\s*(.+)`), "PERSON"},

	// Addresses — highly identifying
	{regexp.MustCompile(`(?i)\*{0,2}(?:Home|Work|Mailing|Current|Physical)?\s*Address:\*{0,2}\s*(.+)`), "ADDRESS"},

	// Dates of birth
	{regexp.MustCompile(`(?i)\*{0,2}(?:Date of )?Birth:\*{0,2}\s*(.+)`), "DOB"},
	{regexp.MustCompile(`(?i)\*{0,2}DOB:\*{0,2}\s*(.+)`), "DOB"},

	// Client/patient IDs in context
	{regexp.MustCompile(`(?i)\*{0,2}(?:Patient|Client|Student)\s+(?:ID|Identifier):\*{0,2}\s*(.+)`), "CLIENT_ID"},
	{regexp.MustCompile(`(?i)\*{0,2}Case\s+(?:Number|#|ID):\*{0,2}\s*(.+)`), "CASE_ID"},
	{regexp.MustCompile(`(?i)\*{0,2}Subscriber (?:Name|SSN):\*{0,2}\s*(.+)`), "PERSON"},
}

// ═══════════ PII TYPE REGISTRY ═══════════

type PIIType struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

var PIITypes = map[string]PIIType{
	"EMAIL":       {Name: "EMAIL", Label: "Email Address"},
	"PHONE":       {Name: "PHONE", Label: "Phone Number"},
	"SSN":         {Name: "SSN", Label: "Social Security Number"},
	"CREDIT_CARD": {Name: "CREDIT_CARD", Label: "Credit Card Number"},
	"IP":          {Name: "IP", Label: "IP Address"},
	"ROUTING":     {Name: "ROUTING", Label: "Bank Routing Number"},
	"ACCOUNT":     {Name: "ACCOUNT", Label: "Bank Account Number"},
	"TAXID":       {Name: "TAXID", Label: "Tax ID"},
	"INSURANCE":   {Name: "INSURANCE", Label: "Insurance ID"},
	"NPI":         {Name: "NPI", Label: "NPI Number"},
	"MRN":         {Name: "MRN", Label: "Medical Record Number"},
	"PERSON":      {Name: "PERSON", Label: "Person Name"},
	"CONTACT":     {Name: "CONTACT", Label: "Emergency Contact"},
	"PROVIDER":    {Name: "PROVIDER", Label: "Healthcare Provider"},
	"ADDRESS":     {Name: "ADDRESS", Label: "Address"},
	"DOB":         {Name: "DOB", Label: "Date of Birth"},
	"CLIENT_ID":   {Name: "CLIENT_ID", Label: "Client/Patient ID"},
	"CASE_ID":     {Name: "CASE_ID", Label: "Case Number"},
	"ORG_CONTACT": {Name: "ORG_CONTACT", Label: "Organization Contact"},
}

// ═══════════ CONFIGURATION ═══════════

type RedactionConfig struct {
	TokenFormat string
	LineLevel   bool
}

var DefaultConfig = RedactionConfig{
	TokenFormat: "[{type}-{shortid}]",
	LineLevel:   true,
}

// ═══════════ RESULT TYPES ═══════════

type PIIItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	TypeLabel string `json:"typeLabel"`
	Original  string `json:"original"`
	Redacted  string `json:"redacted"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Context   string `json:"context"`
}

type RedactionResult struct {
	OriginalText   string            `json:"originalText"`
	RedactedText   string            `json:"redactedText"`
	MarkdownText   string            `json:"markdownText"`
	Items          []PIIItem         `json:"items"`
	TotalFound     int               `json:"totalFound"`
	Mapping        map[string]string `json:"mapping"`
	ReverseMapping map[string]string `json:"reverseMapping"`
}

// ═══════════ CORE REDACTION ENGINE ═══════════

func RedactDocument(content string) RedactionResult {
	return RedactDocumentWithConfig(content, DefaultConfig)
}

func RedactDocumentWithConfig(content string, cfg RedactionConfig) RedactionResult {
	var items []PIIItem
	mapping := make(map[string]string)
	reverseMapping := make(map[string]string)
	counters := make(map[string]int)

	type match struct {
		start    int
		end      int
		piiType  string
		original string
	}

	var matches []match

	findAll := func(re *regexp.Regexp, piiType string) {
		for _, loc := range re.FindAllStringIndex(content, -1) {
			matches = append(matches, match{
				start: loc[0], end: loc[1],
				piiType: piiType, original: content[loc[0]:loc[1]],
			})
		}
	}

	findAll(reEmail, "EMAIL")
	findAll(rePhone, "PHONE")
	findAll(reSSN, "SSN")
	findAll(reCreditCard, "CREDIT_CARD")
	findAll(reIP, "IP")
	findAll(reRouting, "ROUTING")
	findAll(reAccount, "ACCOUNT")
	findAll(reTaxID, "TAXID")
	findAll(reInsuranceID, "INSURANCE")
	findAll(reNPI, "NPI")
	findAll(reMRN, "MRN")

	// Line-level patterns
	if cfg.LineLevel {
		for _, lp := range linePatterns {
			for _, lm := range lp.Re.FindAllStringSubmatchIndex(content, -1) {
				if len(lm) < 4 || lm[2] < 0 {
					continue
				}
				value := strings.TrimSpace(content[lm[2]:lm[3]])
				if value == "" || value == "—" || value == "N/A" || value == "None" {
					continue
				}
				matches = append(matches, match{
					start: lm[2], end: lm[3],
					piiType: lp.Type, original: value,
				})
			}
		}
	}

	// Sort by position reverse
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].end > matches[j].end
		}
		return matches[i].start > matches[j].start
	})

	// Deduplicate overlapping
	var deduped []match
	for _, m := range matches {
		overlaps := false
		for _, d := range deduped {
			if m.start < d.end && m.end > d.start {
				overlaps = true
				break
			}
		}
		if !overlaps {
			deduped = append(deduped, m)
		}
	}

	// Sort reverse for replacement
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].start > deduped[j].start
	})

	// Apply replacements
	redacted := content
	for _, m := range deduped {
		counters[m.piiType]++
		token := formatToken(cfg.TokenFormat, m.piiType, m.original, counters[m.piiType])

		ctxStart := m.start - 40
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxEnd := m.end + 40
		if ctxEnd > len(content) {
			ctxEnd = len(content)
		}
		context := content[ctxStart:ctxEnd]
		if ctxStart > 0 {
			context = "..." + context
		}
		if ctxEnd < len(content) {
			context = context + "..."
		}

		typeInfo := PIITypes[m.piiType]
		items = append(items, PIIItem{
			ID: token, Type: m.piiType, TypeLabel: typeInfo.Label,
			Original: m.original, Redacted: token,
			Start: m.start, End: m.end, Context: context,
		})

		mapping[token] = m.original
		reverseMapping[m.original] = token
		redacted = redacted[:m.start] + token + redacted[m.end:]
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Start < items[j].Start
	})

	return RedactionResult{
		OriginalText: content, RedactedText: redacted, MarkdownText: redacted,
		Items: items, TotalFound: len(items),
		Mapping: mapping, ReverseMapping: reverseMapping,
	}
}

func formatToken(tmpl, piiType, original string, counter int) string {
	h := sha256.Sum256([]byte(original))
	shortID := fmt.Sprintf("%x", h[:3])[:6]
	token := tmpl
	token = strings.ReplaceAll(token, "{type}", piiType)
	token = strings.ReplaceAll(token, "{shortid}", shortID)
	token = strings.ReplaceAll(token, "{counter}", fmt.Sprintf("%d", counter))
	return token
}

// ═══════════ DE-REDACTION ═══════════

func DeRedact(redactedText string, mapping map[string]string) string {
	result := redactedText
	for token, original := range mapping {
		result = strings.ReplaceAll(result, token, original)
	}
	return result
}

func DeRedactPartial(redactedText string, items []PIIItem, allowedTypes map[string]bool) string {
	result := redactedText
	for _, item := range items {
		if allowedTypes[item.Type] {
			result = strings.ReplaceAll(result, item.Redacted, item.Original)
		}
	}
	return result
}

// ═══════════ CUSTOM PATTERNS ═══════════

// CustomPatternRegexPrefix marks session patterns stored as Go regexp (see regen package).
const CustomPatternRegexPrefix = "regex:"

func ApplyCustomPatterns(result *RedactionResult, patterns []string) {
	var literals, regexExprs []string
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, CustomPatternRegexPrefix) {
			regexExprs = append(regexExprs, strings.TrimPrefix(pattern, CustomPatternRegexPrefix))
		} else {
			literals = append(literals, pattern)
		}
	}
	applyLiteralCustomPatterns(result, literals)
	applyRegexCustomPatterns(result, regexExprs)
}

func applyLiteralCustomPatterns(result *RedactionResult, patterns []string) {
	nameCounter := 0
	for _, pattern := range patterns {
		if !strings.Contains(result.RedactedText, pattern) {
			continue
		}
		nameCounter++
		token := fmt.Sprintf("[REDACTED_NAME_%d]", nameCounter)
		result.RedactedText = strings.ReplaceAll(result.RedactedText, pattern, token)
		result.Mapping[token] = pattern
		if result.ReverseMapping == nil {
			result.ReverseMapping = make(map[string]string)
		}
		result.ReverseMapping[pattern] = token

		idx := strings.Index(result.OriginalText, pattern)
		ctx := snippetContext(result.OriginalText, idx, len(pattern))

		result.Items = append(result.Items, PIIItem{
			ID: token, Type: "NAME", TypeLabel: "Person Name",
			Original: pattern, Redacted: token,
			Start: idx, End: idx + len(pattern), Context: ctx,
		})
		result.TotalFound++
	}
}

func applyRegexCustomPatterns(result *RedactionResult, expressions []string) {
	customCounter := 0
	for _, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			continue
		}
		indices := re.FindAllStringIndex(result.RedactedText, -1)
		sort.Slice(indices, func(i, j int) bool {
			return indices[i][0] > indices[j][0]
		})
		for _, loc := range indices {
			matched := result.RedactedText[loc[0]:loc[1]]
			customCounter++
			token := fmt.Sprintf("[REDACTED_CUSTOM_%d]", customCounter)
			result.RedactedText = result.RedactedText[:loc[0]] + token + result.RedactedText[loc[1]:]
			result.Mapping[token] = matched
			if result.ReverseMapping == nil {
				result.ReverseMapping = make(map[string]string)
			}
			result.ReverseMapping[matched] = token

			idx := strings.Index(result.OriginalText, matched)
			if idx < 0 {
				idx = loc[0]
			}
			ctx := snippetContext(result.OriginalText, idx, len(matched))

			result.Items = append(result.Items, PIIItem{
				ID: token, Type: "CUSTOM", TypeLabel: "Custom Pattern",
				Original: matched, Redacted: token,
				Start: idx, End: idx + len(matched), Context: ctx,
			})
			result.TotalFound++
		}
	}
}

func snippetContext(text string, idx, length int) string {
	if idx < 0 {
		idx = 0
	}
	ctxStart := idx - 40
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := idx + length + 40
	if ctxEnd > len(text) {
		ctxEnd = len(text)
	}
	ctx := text[ctxStart:ctxEnd]
	if ctxStart > 0 {
		ctx = "..." + ctx
	}
	if ctxEnd < len(text) {
		ctx = "..." + ctx
	}
	return ctx
}

// ═══════════ KNOWN NAMES ═══════════

var activeKnownNames []string

func SetActiveKnownNames(names []string) {
	activeKnownNames = names
}

func GetActiveKnownNames() []string {
	if len(activeKnownNames) > 0 {
		return activeKnownNames
	}
	return []string{
		"Sarah Mitchell", "James Thornton", "Lisa Thornton",
		"Maria Rodriguez", "Carlos Rodriguez", "David Chen",
		"Priya Patel", "Tom Bradley", "Kevin Walsh",
		"Rachel Foster", "Mike Santos", "Angela Morris",
		"James Wright",
	}
}

func GetMissedNames(originalText string, result *RedactionResult) []string {
	knownNames := GetActiveKnownNames()
	redacted := make(map[string]bool)
	for _, item := range result.Items {
		redacted[item.Original] = true
	}
	var missed []string
	for _, name := range knownNames {
		if !redacted[name] && strings.Contains(originalText, name) {
			missed = append(missed, name)
		}
	}
	return missed
}

// ═══════════ DISPLAY HELPERS ═══════════

func (r RedactionResult) GetMappingDisplay() []string {
	var lines []string
	for token, original := range r.Mapping {
		lines = append(lines, fmt.Sprintf("%s → %s", token, original))
	}
	sort.Strings(lines)
	return lines
}

func (r RedactionResult) FormatMappingForDisplay() string {
	if len(r.Mapping) == 0 {
		return "No PII detected."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d PII items:\n\n", r.TotalFound))
	for _, item := range r.Items {
		b.WriteString(fmt.Sprintf("  %s = \"%s\"\n    ↳ %s\n", item.Redacted, item.Original, item.TypeLabel))
	}
	return b.String()
}
