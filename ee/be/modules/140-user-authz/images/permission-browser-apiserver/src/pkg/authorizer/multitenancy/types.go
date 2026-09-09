/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import "github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

// NamespaceAccessType represents the result of namespace access evaluation.
type NamespaceAccessType int

const (
	// AllNamespacesAllowed means user has no MT restrictions (privileged or no filters).
	AllNamespacesAllowed NamespaceAccessType = iota
	// NoNamespacesAllowed means user has no CAR and is not privileged (deny-by-default).
	NoNamespacesAllowed
	// FilteredAccess means user has CAR with restrictions, each namespace must be checked.
	FilteredAccess
)

// DirectoryEntry is the combined multi-tenancy scope of a subject: the entry of the rules
// directory the webhook enforces, as the library builds it.
type DirectoryEntry = rules.Entry
