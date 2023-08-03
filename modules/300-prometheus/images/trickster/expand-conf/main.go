package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading standard input: %v", err)
	}
	fmt.Fprint(os.Stdout, os.ExpandEnv(string(content)))
}
