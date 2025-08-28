package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"gopkg.in/yaml.v2"
)

func Test1231312(t *testing.T) {
	// Sample JSON string
	jsonString := `{"name": "Alice", "age": 30, "city": "New York", "interests": ["reading", "hiking"]}`

	// Unmarshal JSON into a Go interface{}
	var data interface{}
	err := json.Unmarshal([]byte(jsonString), &data)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	// Marshal the Go data into YAML
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		log.Fatalf("Error marshalling to YAML: %v", err)
	}

	// Print the YAML output
	fmt.Println("Converted YAML:")
	fmt.Println(string(yamlBytes))
}
