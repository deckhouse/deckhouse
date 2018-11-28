package utils

import (
	"sort"
)

// SortReverseByReference returns new array reverse sorted by the order of reference array
func SortReverseByReference(in []string, ref []string) []string {
	res := make([]string, 0)

	inValues := make(map[string]bool, 0)
	for _, v := range in {
		inValues[v] = true
	}
	for _, v := range ref {
		if _, hasKey := inValues[v]; hasKey {
			// prepend
			res = append([]string{v}, res...)
		}
	}

	return res
}

// SortReverse create a copy of in array and sort it in reverse order
func SortReverse(in []string) []string {
	res := make([]string, 0)
	for _, v := range in {
		res = append(res, v)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(res)))

	return res
}

// SortByReference returns new array sorted by the order of reference array
func SortByReference(in []string, ref []string) []string {
	res := make([]string, 0)

	inValues := make(map[string]bool, 0)
	for _, v := range in {
		inValues[v] = true
	}
	for _, v := range ref {
		if _, hasKey := inValues[v]; hasKey {
			res = append(res, v)
		}
	}

	return res
}

// ListSubtract creates a new array from src without items in ignored arrays
func ListSubtract(src []string, ignored ...[]string) (result []string) {
	ignoredMap := make(map[string]bool, 0)
	for _, arr := range ignored {
		for _, v := range arr {
			ignoredMap[v] = true
		}
	}

	for _, v := range src {
		if _, ok := ignoredMap[v]; !ok {
			result = append(result, v)
		}
	}
	return
}

// ListIntersection returns an intersection between arrays with unique values
func ListIntersection(arrs ...[]string) (result []string) {
	if len(arrs) == 0 {
		return
	}

	// count each item in arrays
	m := make(map[string]int)
	for _, a := range arrs {
		for _, v := range a {
			if _, ok := m[v]; ok {
				m[v] = m[v] + 1
			} else {
				m[v] = 1
			}
		}
	}

	for k, v := range m {
		if v == len(arrs) {
			result = append(result, k)
		}
	}

	return
}

// ListFullyIn returns whether all arr items contains in ref array
func ListFullyIn(arr []string, ref []string) bool {
	res := true

	m := make(map[string]bool, 0)
	for _, v := range ref {
		m[v] = true
	}

	for _, v := range arr {
		if _, ok := m[v]; !ok {
			return false
		}
	}

	return res
}
