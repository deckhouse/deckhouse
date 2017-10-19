package main

func mergeTwoValues(A map[string]interface{}, B map[string]interface{}) map[string]interface{} {
	// TODO: deep merge

	res := make(map[string]interface{})

	for key, value := range A {
		res[key] = value
	}
	for key, value := range B {
		res[key] = value
	}

	return res
}

func MergeValues(ValuesArr ...map[string]interface{}) map[string]interface{} {
	res := make(map[string]interface{})

	for _, values := range ValuesArr {
		res = mergeTwoValues(res, values)
	}

	return res
}
