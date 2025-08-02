package config

import "strings"

func pluralize(word string) string {
	lower := strings.ToLower(word)
	switch {
	case strings.HasSuffix(lower, "s"),
		strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"):
		return word + "es"
	default:
		return word + "s"
	}
}
