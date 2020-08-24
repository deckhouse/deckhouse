package log

import (
	"bytes"
	"encoding/json"
)

type Logger interface {
	LogProcess(string, func() error) error
	LogInfoF(format string, a ...interface{})
	LogInfoLn(a ...interface{})
	LogErrorF(format string, a ...interface{})
	LogErrorLn(a ...interface{})
}

func PrettyJSON(content []byte) string {
	result := &bytes.Buffer{}
	if err := json.Indent(result, content, "", "  "); err != nil {
		panic(err)
	}

	return result.String()
}
