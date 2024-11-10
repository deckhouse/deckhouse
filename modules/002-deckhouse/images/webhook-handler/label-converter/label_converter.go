package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	if len(os.Args) != 2 {
		help()
		os.Exit(1)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Printf("Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	var out []byte
	switch os.Args[1] {
	case "-h", "--help":
		help()
		os.Exit(0)
	case "--to-equality":
		out, err = toEquality(data)
		if err != nil {
			fmt.Printf("Error converting to-equality: %v\n", err)
			os.Exit(1)
		}
	case "--to-set":
		out, err = toSet(data)
		if err != nil {
			fmt.Printf("Error converting to-set: %v\n", err)
			os.Exit(1)
		}
	default:
		help()
		os.Exit(1)
	}

	os.Stdout.Write(out)
}

func help() {
	fmt.Println(`Usage: label-converter [OPTION]
Converts kubernetes labels between equality-based and set-based form.
Reads input from STDIN.

  --help			display this help and exit.
  --to-equality		converts from set-based form to equality-based form.
  --to-set			covnerts from equality-based form to set-based form.`)
}

func toSet(label []byte) ([]byte, error) {
	l, err := metav1.ParseToLabelSelector(string(label))
	if err != nil {
		return nil, fmt.Errorf("Error parsing label selector: %v", err)
	}
	out, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling label: %v", err)
	}
	return out, nil
}

func toEquality(label []byte) ([]byte, error) {
	ls := &metav1.LabelSelector{}
	err := json.Unmarshal(label, ls)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling label: %v", err)
	}
	return []byte(metav1.FormatLabelSelector(ls)), nil
}
