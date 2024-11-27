/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import "embed"

//go:embed templates
var templatesFS embed.FS

type templateName string

const (
	authConfigTemplateName         templateName = "templates/auth/config.yaml.tpl"
	distributionConfigTemplateName templateName = "templates/distribution/config.yaml.tpl"
	registryStaticPodTemplateName  templateName = "templates/static_pods/system-registry.yaml.tpl"
)

func getTemplateContent(name templateName) ([]byte, error) {
	return templatesFS.ReadFile(string(name))
}
