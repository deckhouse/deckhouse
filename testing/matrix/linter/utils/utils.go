package utils

import "github.com/ghodss/yaml"

func ModuleEnabled(values, moduleName string) bool {
	var v struct {
		Global struct{ EnabledModules []string }
	}
	err := yaml.Unmarshal([]byte(values), &v)
	if err != nil {
		panic("unable to parse global.enabledModules values section")
	}

	for _, module := range v.Global.EnabledModules {
		if module == moduleName {
			return true
		}
	}
	return false
}
