package main

import (
	"encoding/gob"
	"encoding/xml"
	"log"
	"os"

	"github.com/deckhouse/deckhouse/modules/500-operator-trivy/images/bdu-exporter/types"
)

func main() {
	sourcePath := os.Args[1]
	destinationPath := os.Args[2]

	source, err := os.Open(sourcePath)
	if err != nil {
		log.Fatal(err)
	}
	destination, err := os.Create(destinationPath)
	if err != nil {
		log.Fatal(err)
	}

	var vulns types.Vulnerabilities
	decoder := xml.NewDecoder(source)
	err = decoder.Decode(&vulns)
	if err != nil {
		log.Fatal(err)
	}

	gobEncoder := gob.NewEncoder(destination)

	err = gobEncoder.Encode(vulns)
	if err != nil {
		log.Fatal(err)
	}
}
