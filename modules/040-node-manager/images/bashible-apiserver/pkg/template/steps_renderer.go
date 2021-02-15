package template

import (
	"fmt"
)

func NewStepsRenderer(bashibleContext Context, rootDir string, target string, nameMapper NameMapper) *StepsRenderer {
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
	contextKey, err := s.contextName(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get context configMapKey: %v", err)
	}
	context, err := s.bashibleContext.Get(contextKey)
	if err != nil {
		return nil, fmt.Errorf("cannot get context data: %v", err)
	}
	return context, nil
}

// pick $.cloudProvider.type as a string
// TODO better be known in advance from the config
func (s StepsRenderer) getProviderType(templateContext map[string]interface{}) (string, error) {
	cloudProvider, ok := templateContext["cloudProvider"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("cloudProvider is not map[string]interface{}")
	}
	providerType, ok := cloudProvider["type"].(string)
	if !ok {
		return "", fmt.Errorf("cloudProvider.type is not a string")
	}
	return providerType, nil
}
