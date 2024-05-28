/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

func IsStringInSlice(value string, slice *[]string) bool {
	if slice == nil {
		return false
	}
	for _, str := range *slice {
		if str == value {
			return true
		}
	}
	return false
}
