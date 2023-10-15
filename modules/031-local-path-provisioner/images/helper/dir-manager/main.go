package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	// flags
	dirMode     string
	path        string
	sizeInBytes string
	action      string
)

const (
	SETUP    = "create"
	TEARDOWN = "delete"
)

func init() {
	flag.StringVar(&path, "p", "", "Absolute path")
	flag.StringVar(&sizeInBytes, "s", "", "Size in bytes")
	flag.StringVar(&dirMode, "m", "", "Dir mode")
	flag.StringVar(&action, "a", "", fmt.Sprintf("Action name. Can be '%s' or '%s'", SETUP, TEARDOWN))
}

func main() {
	flag.Parse()
	if action != SETUP && action != TEARDOWN {
		fmt.Fprintf(os.Stderr, "Error. Incorrect action: %s", action)
		os.Exit(1)
	}

	if path == "" {
		os.Stderr.WriteString("Error. Empty string")
		os.Exit(2)
	}

	if path == "/" {
		os.Stderr.WriteString("Error. Path cannot be '/'")
		os.Exit(3)
	}

	fmt.Printf("Do action '%s' for path %s \n", action, path)

	if action == TEARDOWN {
		err := os.RemoveAll(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error. Cannot remove directory %s: %s", path, err)
			os.Exit(4)
		}
		return
	}

	err := os.MkdirAll(path, 0777)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error. Cannot create directory %s: %s", path, err)
		os.Exit(5)
	}
}
