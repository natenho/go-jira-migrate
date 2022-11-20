package internal

import "strings"

func RemoveCharacters(input string, characters string) string {
	filter := func(r rune) rune {
		if strings.ContainsRune(characters, r) {
			return -1
		}
		return r
	}

	return strings.Map(filter, input)
}
