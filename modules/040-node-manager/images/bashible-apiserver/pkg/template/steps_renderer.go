package template

import (
	"fmt"
)

const versionMap = "versionMap"

func NewStepsRenderer(bashibleContext Context, rootDir, target string, nameMapper NameMapper) *StepsRenderer {
	return &StepsRenderer{
		bashibleContext: bashibleContext,
		rootDir:         rootDir,
		contextName:     nameMapper,
		target:          target,
	}
}

type StepsRenderer struct {
	bashibleContext Context
	rootDir         string
	contextName     NameMapper
	target          string
}

// Render renders single script content by name which is expected to be of form {os}.{target}
func (s StepsRenderer) Render(name string) (map[string]string, error) {
	templateContext, err := s.getContext(name)
	if err != nil {
		return nil, err
	}
	providerType, err := s.getProviderType(templateContext)
	if err != nil {
		return nil, err
	}
	stepsStorage := NewStepsStorage(s.rootDir, providerType, s.target)
	return stepsStorage.Render(templateContext)
}

func (s StepsRenderer) getContext(name string) (map[string]interface{}, error) {
	fullContext := make(map[string]interface{})
	contextKey, err := s.contextName(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get context secretKey: %v", err)
	}
	context, err := s.bashibleContext.Get(contextKey)
	if err != nil {
		return nil, fmt.Errorf("cannot get context data: %v", err)
	}
	versionMapContext, err := s.bashibleContext.Get(versionMap)
	if err != nil {
		return nil, fmt.Errorf("cannot get versionMap context data: %v", err)
	}
	for k, v := range versionMapContext {
		fullContext[k] = v
	}
	for k, v := range context {
		fullContext[k] = v
	}

	return fullContext, nil
}

// getProviderType picks $.cloudProvider.type as a string. When we cannot parse this field, it can mean that the
// node group is static, e.g. not in the cloud.
// TODO better be known in advance from the config
func (s StepsRenderer) getProviderType(templateContext map[string]interface{}) (string, error) {
	cloudProvider, ok := templateContext["cloudProvider"].(map[string]interface{})
	if !ok {
		// absent cloud provider means static nodes
		return "", nil
	}
	providerType, ok := cloudProvider["type"].(string)
	if !ok {
		return "", fmt.Errorf("cloudProvider.type is not a string")
	}
	return providerType, nil
}
