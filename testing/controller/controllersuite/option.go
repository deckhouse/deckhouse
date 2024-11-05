package controllersuite

import "io"

type SuiteOption func(suite *Suite)

func WithLogOutput(writer io.Writer) SuiteOption {
	return func(suite *Suite) {
		suite.logOutput = writer
	}
}
