package main

import (
	"github.com/romana/rlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

func main() {
	rlog.Infof("Hello")

	valuesYaml, err := ioutil.ReadFile("values.yaml")
	if err != nil {
		rlog.Errorf("Cannot read values.yaml: %s", err)
		os.Exit(1)
	}

	var res map[interface{}]interface{}
	err = yaml.Unmarshal(valuesYaml, &res)
	if err != nil {
		rlog.Errorf("Bad values.yaml: %s", err)
		os.Exit(1)
	}

	rlog.Infof("RES: %v", res)
	rlog.Infof("registry.secret_name=%s", res["registry"].(map[interface{}]interface{})["secret_name"])

	rlog.Infof("Goodbye")
}
