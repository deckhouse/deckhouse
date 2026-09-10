/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import "github.com/deckhouse/deckhouse/go_lib/user-authz/rules"

// NamespaceAccessType represents the result of namespace access evaluation.
type NamespaceAccessType int

const (
	// AllNamespacesAllowed: a rule names this subject and imposes no namespace filter.
	AllNamespacesAllowed NamespaceAccessType = iota
	// NoNamespacesAllowed: no rule names this subject at all.
	//
	// The name reads as a verdict and is not one. Both callers treat it exactly like
	// AllNamespacesAllowed, and deliberately: a subject without a ClusterAuthorizationRule is the
	// norm under the newer role model, where access comes from RoleBindings, and zeroing them out
	// of a report would hide access they genuinely have. Multi-tenancy simply has no opinion here,
	// which is also what the enforcement webhook answers for the same subject.
	NoNamespacesAllowed
	// FilteredAccess: a rule names this subject and limits it, so each namespace must be checked
	// against the returned entry.
	FilteredAccess
)

// DirectoryEntry is the combined multi-tenancy scope of a subject: the entry of the rules
// directory the webhook enforces, as the library builds it.
type DirectoryEntry = rules.Entry
