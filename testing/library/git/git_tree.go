package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
	if err != nil {
		return nil, fmt.Errorf("failed to run \"git\" command: %s\n\n%s", err, err.(*exec.ExitError).Stderr)
	}

	parsedObjects := parseLsTreeOutput(bytes.NewReader(output))

	return parsedObjects, nil
}

func parseLsTreeOutput(reader io.Reader) (objects []TreeObject) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		splitted := strings.Fields(line)
		object := TreeObject{
			Mode:   splitted[0],
			Type:   splitted[1],
			Object: splitted[2],
			File:   splitted[3],
		}
		objects = append(objects, object)
	}

	return
}
