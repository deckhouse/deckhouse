/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package consts

const (
	ProjectRequireSyncAnnotation = "projects.deckhouse.io/require-sync"
	ProjectRequireSyncKeyTrue    = "true"
	ProjectRequireSyncKeyFalse   = "false"
)

const (
	ProjectLabel         = "projects.deckhouse.io/project"
	ProjectTemplateLabel = "projects.deckhouse.io/project-template"
)

const (
	HeritageLabel        = "heritage"
	MultitenancyHeritage = "multitenancy-manager"
	DeckhouseHeritage    = "deckhouse"
)
const ProjectFinalizer = "projects.deckhouse.io/project-exists"

const ReleaseHashLabel = "hashsum"
