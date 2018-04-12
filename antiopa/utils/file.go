package utils

import (
	"os"
)

func IsFileExecutable(f os.FileInfo) bool {
	return f.Mode()&0111 != 0
}
