package util

import (
	"math/rand"
	"time"
)

func RandomStrElement(list []string) (string, int) {
	// we silent gosec linter here
	// because we do not need security random number
	// for choice random element
	s := rand.NewSource(time.Now().Unix()) //nolint:gosec
	r := rand.New(s)                       //nolint:gosec
	indx := r.Intn(len(list))

	return list[indx], indx
}

func ExcludeElementFromSlice(list []string, elem string) []string {
	indx := -1
	for i, v := range list {
		if v == elem {
			indx = i
			break
		}
	}

	if indx >= 0 {
		firstPart := list[:indx]
		// need tmp slice because
		// res := append(list[:indx], list[indx+1:]...)
		// affect source list
		tmp := make([]string, len(firstPart))
		copy(tmp, firstPart)
		res := append(tmp, list[indx+1:]...)

		return res
	}

	return list
}
