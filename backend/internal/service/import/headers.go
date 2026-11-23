package service

import "strings"

// normalizeHeader lower-cases a column header and turns spaces and hyphens
// into underscores.
func normalizeHeader(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}
