package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func chown(path string) error {
	fmt.Println(path)
	uid := 64535
	gid := 64535
	err := os.Chown(path, uid, gid)
	if err != nil {
		fmt.Println(err)
	}
	return nil
}

func main() {
	walkDirFunc := func(path string, d fs.DirEntry, _ error) error {
		return chown(path)
	}

	for _, path := range os.Args {
		err := filepath.WalkDir(path, walkDirFunc)
		if err != nil {
			fmt.Println(err)
		}
	}
}
