package main

func mergeTwoValues(A map[string]interface{}, B map[string]interface{}) map[string]interface{} {
	// FIXME
	return A
}

func MergeValues(ValuesArr map[string]interface{}...) map[string]interface{} {
	// FIXME

	res := make(map[string]interface{})

	for _, values := ValuesArr {
		res = MergeValues(res, values)
	}

	return res
}