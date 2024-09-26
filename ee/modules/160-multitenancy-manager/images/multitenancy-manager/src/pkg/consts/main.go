/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package consts

const ProjectRequireSyncAnnotation = "projects.deckhouse.io/require-sync"

const ProjectFinalizer = "projects.deckhouse.io/project-exists"

const (
	ProjectLabel         = "projects.deckhouse.io/project"
	ProjectTemplateLabel = "projects.deckhouse.io/project-template"
	ProjectVirtualLabel  = "projects.deckhouse.io/virtual-project"
)

const (
	HeritageLabel        = "heritage"
	MultitenancyHeritage = "multitenancy-manager"
	DeckhouseHeritage    = "deckhouse"
)

const HelmDriver = "secret"

const ReleaseHashLabel = "hashsum"

const (
	DeckhouseProjectName = "deckhouse"
	DefaultProjectName   = "default"

	VirtualTemplate = "virtual"
)
const (
	DeckhouseNamespacePrefix  = "d8-"
	KubernetesNamespacePrefix = "kube-"
)
