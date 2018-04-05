package utils

type ModuleValues struct {
	IsEnabled bool
	Values    map[interface{}]interface{}
}

func NewModuleValues(interface{}) (*ModuleValues, error) {
	/*
		if moduleEnabled, isBool := valueData.(bool); isBool {
			moduleConfig.IsEnabled = moduleEnabled
		} else {
			moduleValues, moduleValuesOk := valueData.(map[interface{}]interface{})
			if !moduleValuesOk {
				return nil, fmt.Errorf("expected map or bool, got: %s")
			}
			moduleConfig.Values = moduleValues
		}
	*/
	return nil, nil
}

func FormatValues(map[interface{}]interface{}) (map[interface{}]interface{}, error) { return nil, nil }
