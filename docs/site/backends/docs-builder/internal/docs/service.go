package docs

import (
	"regexp"
	"sync/atomic"
)

var docConfValuesRegexp = regexp.MustCompile(`^openapi/doc-.*-config-values\.yaml$`)
var assembleErrorRegexp = regexp.MustCompile(`error building site: assemble: (\x1b\[1;36m)?"(?P<path>.+):(?P<line>\d+):(?P<column>\d+)"(\x1b\[0m)?:`)

type Service struct {
	baseDir              string
	destDir              string
	isReady              atomic.Bool
	channelMappingEditor *channelMappingEditor
}

func NewService(baseDir, destDir string, highAvailability bool) *Service {
	svc := &Service{
		baseDir:              baseDir,
		destDir:              destDir,
		channelMappingEditor: newChannelMappingEditor(baseDir),
	}

	if !highAvailability {
		svc.isReady.Store(true)
	}

	return svc
}

func (svc *Service) IsReady() bool {
	return svc.isReady.Load()
}
