package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type TreeObject struct {
	Mode   string
	Type   string
	Object string
	File   string
}

func ListTreeObjects(path string) ([]TreeObject, error) {
	cmd := exec.Command("git", "ls-tree", "@", "--", ".")
	cmd.Dir = path

	output, err := cmd.Output()

	switch e := err.(type) {
	case *exec.ExitError:
		return nil, fmt.Errorf("failed to run \"git\" command: %s\n\n%s", err, e.Stderr)
	case *os.PathError:
		// images directory does not exist in module folder, return an empty images array
		return []TreeObject{}, nil
	case nil:
		parsedObjects := parseLsTreeOutput(bytes.NewReader(output))
		return parsedObjects, nil
	default:
		return nil, fmt.Errorf("unknown error occured while reading images: %v", err)
	}
}

func parseLsTreeOutput(reader io.Reader) (objects []TreeObject) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		object := TreeObject{Mode: line[0], Type: line[1], Object: line[2], File: line[3]}
		objects = append(objects, object)
	}

	return
}
