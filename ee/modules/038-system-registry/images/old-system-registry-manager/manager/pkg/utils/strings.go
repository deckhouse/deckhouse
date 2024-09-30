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

func RemoveStringFromSlice(strToRemove string, slice []string) []string {
	for i, v := range slice {
		if v == strToRemove {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func InsertString(strToInsert string, slice []string) []string {
	for _, v := range slice {
		if v == strToInsert {
			return slice
		}
	}
	return append(slice, strToInsert)
}
