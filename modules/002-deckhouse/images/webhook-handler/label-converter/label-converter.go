package main

import (
	"encoding/json"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: ./label-converter <label expression>")
		os.Exit(1)
	}
	label := os.Args[1]
	l, err := metav1.ParseToLabelSelector(label)
	if err != nil {
		fmt.Println("Error parsing label selector:", err)
		os.Exit(1)
	}
	out, err := json.Marshal(l)
	if err != nil {
		fmt.Println("Error marshalling label:", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}
