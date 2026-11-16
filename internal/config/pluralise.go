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
	case len(lower) > 1 && strings.HasSuffix(lower, "y") && !strings.ContainsRune("aeiou", rune(lower[len(lower)-2])):
		return word[:len(word)-1] + "ies"
	default:
		return word + "s"
	}
}
