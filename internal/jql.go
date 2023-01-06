package internal

import (
	"fmt"
	"regexp"
	"strings"
)

func SanitizeJQL(projectKey, jql string) string {
	jql = regexp.MustCompile(`(?i)project\s?=\s?`).ReplaceAllString(jql, "")
	jql = regexp.MustCompile(`(?i)order\s+by.*`).ReplaceAllString(jql, "")

	return fmt.Sprintf("project = %s AND (%s) ORDER BY key ASC", projectKey, strings.TrimSpace(jql))
}

func SanitizeTermForJQL(input string) string {
	input = strings.ReplaceAll(input, " - ", " ")
	input = strings.ReplaceAll(input, "- ", " ")
	input = strings.ReplaceAll(input, " -", " ")
	input = strings.ReplaceAll(input, "\t", "\\t")

	const notAllowedSearchChars string = `-["]()?*+`

	filter := func(r rune) rune {
		if strings.ContainsRune(notAllowedSearchChars, r) {
			return ' '
		}
		return r
	}

	return strings.Map(filter, input)
}
