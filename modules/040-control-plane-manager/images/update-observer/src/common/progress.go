package common

import "fmt"

func CalculateProgress(current, total int) string {
	if current == 0 {
		return "0%"
	}
	p := (total * 100) / current

	return fmt.Sprint(min(p, 100), "%")
}
