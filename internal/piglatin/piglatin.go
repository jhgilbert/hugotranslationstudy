package piglatin

import (
	"strings"
	"unicode"
)

// pigWord converts a single word to Pig Latin.
func pigWord(word string) string {
	if word == "" {
		return ""
	}
	runes := []rune(word)
	isUpper := unicode.IsUpper(runes[0])
	lowerWord := strings.ToLower(word)

	vowels := "aeiou"
	if strings.ContainsRune(vowels, rune(lowerWord[0])) {
		res := lowerWord + "way"
		if isUpper {
			return strings.Title(res)
		}
		return res
	}

	// find first vowel
	i := 0
	for ; i < len(lowerWord); i++ {
		if strings.ContainsRune(vowels, rune(lowerWord[i])) {
			break
		}
	}
	if i == len(lowerWord) {
		// no vowel found
		return lowerWord + "ay"
	}
	res := lowerWord[i:] + lowerWord[:i] + "ay"
	if isUpper {
		return strings.Title(res)
	}
	return res
}

// toPigLatin converts an entire string, word by word, to Pig Latin.
func ToPigLatin(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r)
	})

	// Split with delimiters preserved
	var result strings.Builder
	start := 0
	for _, w := range words {
		idx := strings.Index(s[start:], w)
		if idx >= 0 {
			// write any punctuation before this word
			result.WriteString(s[start : start+idx])
			result.WriteString(pigWord(w))
			start += idx + len(w)
		}
	}
	// trailing punctuation
	if start < len(s) {
		result.WriteString(s[start:])
	}
	return result.String()
}