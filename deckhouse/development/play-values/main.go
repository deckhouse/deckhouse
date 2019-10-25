package main

import (
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func main() {
	log.Infof("Hello")

	valuesYaml, err := ioutil.ReadFile("values.yaml")
	if err != nil {
		log.Errorf("Cannot read values.yaml: %s", err)
		os.Exit(1)
	}

	var res map[interface{}]interface{}
	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		log.Errorf("Bad values.yaml: %s", err)
		os.Exit(1)
	}

	log.Infof("RES: %v", res)
	log.Infof("registry.secret_name=%s", res["registry"].(map[interface{}]interface{})["secret_name"])

	log.Infof("Goodbye")
}
