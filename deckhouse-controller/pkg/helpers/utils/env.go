package utils

import (
	"fmt"
	"os"
)

func GetEnvOrDie(envName string) (string, error) {
	value, ok := os.LookupEnv(envName)
	if !ok {
		return "", fmt.Errorf("env \"%s\" is not defined", envName)
	}
	return value, nil
}
