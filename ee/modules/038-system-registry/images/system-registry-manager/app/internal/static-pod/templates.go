/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package static_pod

import "embed"

//go:embed templates
var templatesFS embed.FS

type templateName string

const (
	authTemplateName         templateName = "templates/auth_config/config.yaml.tpl"
	distributionTemplateName templateName = "templates/distribution_config/config.yaml.tpl"
	staticPodTemplateName    templateName = "templates/static_pods/system-registry.yaml.tpl"
)

func getTemplateContent(name templateName) ([]byte, error) {
	return templatesFS.ReadFile(string(name))
}
