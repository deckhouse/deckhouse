/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/resolver"
)

const (
	roleAccessResource = "roleaccessreports"

	// Bounds on the selection. Both fan out into work inside the resolver, and
	// neither has a legitimate use at this size: a cluster ships tens of roles,
	// not thousands.
	maxRequestedRoleNames  = 500
	maxRequestedRoleScopes = 32
)

// knownRoleScopes are the scopes a primary-model role can carry.
var knownRoleScopes = []string{"namespace", "project", "subsystem", "system"}

// RoleAccessReporter builds the catalogue of what roles grant. It is
// implemented by *resolver.RoleAccessResolver.
type RoleAccessReporter interface {
	Report(ctx context.Context, req resolver.RoleAccessRequest) (v1alpha1.RoleAccessReportStatus, error)
}

// RoleAccessStorage implements the REST storage for RoleAccessReport: an
// ephemeral, create-only, cluster-scoped resource answering "what does this
// role grant".
type RoleAccessStorage struct {
	reporter RoleAccessReporter
}

// NewRoleAccessStorage creates a new RoleAccessStorage.
func NewRoleAccessStorage(reporter RoleAccessReporter) *RoleAccessStorage {
	return &RoleAccessStorage{reporter: reporter}
}

//nolint:misspell // Creater is the correct interface name in k8s.io/apiserver
var _ rest.Creater = &RoleAccessStorage{}
var _ rest.Scoper = &RoleAccessStorage{}
var _ rest.Storage = &RoleAccessStorage{}
var _ rest.SingularNameProvider = &RoleAccessStorage{}

// New returns a new RoleAccessReport.
func (s *RoleAccessStorage) New() runtime.Object {
	return &v1alpha1.RoleAccessReport{}
}

// Destroy cleans up resources on shutdown.
func (s *RoleAccessStorage) Destroy() {}

// NamespaceScoped returns false: the roles it reports on are cluster-scoped
// objects.
func (s *RoleAccessStorage) NamespaceScoped() bool {
	return false
}

// GetSingularName returns the singular name of the resource.
func (s *RoleAccessStorage) GetSingularName() string {
	return "roleaccessreport"
}

// Create builds the catalogue for the requested roles.
//
// Unlike the subject reports, this one has no self/non-self split: it says what
// the platform's roles grant, which is the same answer for everybody and is
// already readable through the ClusterRoles themselves. Who may ask is decided
// by RBAC on this resource alone.
func (s *RoleAccessStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	report, ok := obj.(*v1alpha1.RoleAccessReport)
	if !ok {
		return nil, apierrors.NewBadRequest("object is not a RoleAccessReport")
	}

	if err := validateRoleAccessSpec(&report.Spec); err != nil {
		return nil, err
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	status, err := s.reporter.Report(ctx, buildRoleAccessRequest(&report.Spec))
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("building role access report: %w", err))
	}

	report.Status = status

	klog.V(4).Infof("RoleAccessReport: model %q -> %d roles", status.Snapshot.Model, len(status.Roles))

	return report, nil
}

// validateRoleAccessSpec rejects a request that cannot be answered, so the
// caller gets a 400 naming the field instead of an empty report.
func validateRoleAccessSpec(spec *v1alpha1.RoleAccessReportSpec) error {
	switch spec.Model {
	case "", resolver.RoleModelPrimary, resolver.RoleModelLegacy:
	default:
		return apierrors.NewBadRequest(fmt.Sprintf("spec.model must be %q or %q", resolver.RoleModelPrimary, resolver.RoleModelLegacy))
	}

	if len(spec.Roles.Names) > maxRequestedRoleNames {
		return apierrors.NewBadRequest(fmt.Sprintf("spec.roles.names exceeds the limit of %d entries", maxRequestedRoleNames))
	}
	if len(spec.Roles.Scopes) > maxRequestedRoleScopes {
		return apierrors.NewBadRequest(fmt.Sprintf("spec.roles.scopes exceeds the limit of %d entries", maxRequestedRoleScopes))
	}

	for _, scope := range spec.Roles.Scopes {
		if !slices.Contains(knownRoleScopes, scope) {
			return apierrors.NewBadRequest(fmt.Sprintf("spec.roles.scopes contains an unknown scope %q", scope))
		}
	}

	// A selection that belongs to the other model is a mistake worth naming:
	// silently ignoring it would answer a question the caller did not ask.
	if spec.Model == resolver.RoleModelLegacy && len(spec.Roles.Scopes) > 0 {
		return apierrors.NewBadRequest("spec.roles.scopes applies to the primary model only")
	}
	if spec.Model != resolver.RoleModelLegacy && len(spec.Roles.AccessLevels) > 0 {
		return apierrors.NewBadRequest("spec.roles.accessLevels applies to the legacy model only")
	}

	return nil
}

func buildRoleAccessRequest(spec *v1alpha1.RoleAccessReportSpec) resolver.RoleAccessRequest {
	return resolver.RoleAccessRequest{
		Model:              spec.Model,
		Names:              spec.Roles.Names,
		Scopes:             spec.Roles.Scopes,
		AccessLevels:       spec.Roles.AccessLevels,
		ExpandWildcards:    boolValue(spec.ExpandWildcards, true),
		IncludeComposition: boolValue(spec.IncludeComposition, false),
	}
}

// boolValue reads an optional flag with its default.
func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}

	return *value
}
