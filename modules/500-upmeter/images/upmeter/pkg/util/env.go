package util

import (
	"os"
	"strconv"
)

func GetenvInt64(name string) int {
	s := os.Getenv(name)
	if s == "" || s == "0" {
		return 0
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
