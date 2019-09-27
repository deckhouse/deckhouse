package main

import (
	"fmt"
	"flag"
	"os"
	"bufio"
	"log"
	"strings"
	"k8s.io/apimachinery/pkg/api/resource"
	"time"
)

func main() {
	mode := flag.String("mode", "", "converter mode can be: duration/kube-resource-unit")
	flag.Parse()
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed to read stdin: %s", err)
	}
	input = strings.TrimSuffix(input, "\n")
	if *mode == "duration" {
		bro, err := time.ParseDuration(input)
		if err != nil {
			log.Fatal("failed to parse: %s", err)
		}
		fmt.Println(bro.Seconds())
	} else if *mode == "kube-resource-unit" {
		quantity := resource.MustParse(input)
		fmt.Println(quantity.Value())
	} else {
		fmt.Println("Unknonw mode")
	}
}
