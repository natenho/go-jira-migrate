package internal

import (
	"golang.org/x/exp/slices"
)

func SliceFind[E any](s []E, f func(E) bool) (E, bool) {
	matchIndex := slices.IndexFunc(s, f)
	if matchIndex > -1 {
		return s[matchIndex], true
	}
	var null E
	return null, false
}
