package main

import (
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"gopkg.in/yaml.v3"
)

func main() {
	content, err := os.ReadFile("../../module.yaml")
	if err != nil {
		fmt.Println(err)
		return
	}

	def := new(types.Definition)
	if err := yaml.Unmarshal(content, def); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(def.Requirements.Bootstrapped)
}
