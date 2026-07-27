/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/resolver"
)

const (
	subjectAccessResource = "subjectaccessreports"

	// Requests are bounded independently of the generic apiserver's byte limit:
	// both fields fan out into work inside the resolver.
	maxRequestedNamespaces = 500
	maxRequestedGroups     = 200
)

// serviceAccountUserPrefix is how a ServiceAccount identity appears in a
// request: system:serviceaccount:<namespace>:<name>.
const serviceAccountUserPrefix = "system:serviceaccount:"

// SubjectAccessReporter builds the access report of a subject. It is
// implemented by *resolver.SubjectAccessResolver.
type SubjectAccessReporter interface {
	Report(ctx context.Context, req resolver.SubjectAccessRequest) (v1alpha1.SubjectAccessReportStatus, error)
}

// SubjectAccessStorage implements the REST storage for SubjectAccessReport: an
// ephemeral, create-only, cluster-scoped resource answering "what is this
// subject allowed to do".
type SubjectAccessStorage struct {
	reporter   SubjectAccessReporter
	authorizer authorizer.Authorizer
}

// NewSubjectAccessStorage creates a new SubjectAccessStorage.
func NewSubjectAccessStorage(reporter SubjectAccessReporter, auth authorizer.Authorizer) *SubjectAccessStorage {
	return &SubjectAccessStorage{reporter: reporter, authorizer: auth}
}

//nolint:misspell // Creater is the correct interface name in k8s.io/apiserver
var _ rest.Creater = &SubjectAccessStorage{}
var _ rest.Scoper = &SubjectAccessStorage{}
var _ rest.Storage = &SubjectAccessStorage{}
var _ rest.SingularNameProvider = &SubjectAccessStorage{}

// New returns a new SubjectAccessReport.
func (s *SubjectAccessStorage) New() runtime.Object {
	return &v1alpha1.SubjectAccessReport{}
}

// Destroy cleans up resources on shutdown.
func (s *SubjectAccessStorage) Destroy() {}

// NamespaceScoped returns false: the report spans the whole cluster and carries
// its namespace scope inside the spec.
func (s *SubjectAccessStorage) NamespaceScoped() bool {
	return false
}

// GetSingularName returns the singular name of the resource.
func (s *SubjectAccessStorage) GetSingularName() string {
	return "subjectaccessreport"
}

// Create builds the report for the requested subject.
func (s *SubjectAccessStorage) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	report, ok := obj.(*v1alpha1.SubjectAccessReport)
	if !ok {
		return nil, apierrors.NewBadRequest("object is not a SubjectAccessReport")
	}

	if err := validateSubjectAccessSpec(&report.Spec); err != nil {
		return nil, err
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	caller, ok := request.UserFrom(ctx)
	if !ok {
		// The generic apiserver always populates the user; its absence is a
		// server-side invariant violation, not a client input error.
		return nil, apierrors.NewInternalError(errors.New("no user info in context"))
	}

	req, err := s.buildRequest(ctx, caller, &report.Spec)
	if err != nil {
		return nil, err
	}

	status, err := s.reporter.Report(ctx, req)
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("building subject access report: %w", err))
	}

	report.Status = status

	klog.V(4).Infof("SubjectAccessReport: %s %q -> %d role assignments, %d scopes",
		req.Subject.Kind, req.Subject.Name, len(status.RoleAssignments), len(status.Scopes))

	return report, nil
}

// buildRequest resolves the report input, applying defaults and the non-self
// authorization gate.
func (s *SubjectAccessStorage) buildRequest(ctx context.Context, caller user.Info, spec *v1alpha1.SubjectAccessReportSpec) (resolver.SubjectAccessRequest, error) {
	req := resolver.SubjectAccessRequest{
		ExtraGroups:     spec.Groups,
		Namespaces:      spec.Namespaces,
		ResolveGroups:   spec.ResolveGroups == nil || *spec.ResolveGroups,
		ExpandWildcards: spec.ExpandWildcards == nil || *spec.ExpandWildcards,
	}

	if spec.Subject == nil {
		// Self mode: the caller's own token is the source of truth for its
		// groups, so they are taken as-is instead of being looked up.
		req.Subject = selfSubject(caller)
		req.CallerGroups = caller.GetGroups()
		req.ResolveGroups = false

		return req, nil
	}

	if err := s.authorizeNonSelfReport(ctx, caller); err != nil {
		return resolver.SubjectAccessRequest{}, err
	}

	req.Subject = *spec.Subject

	return req, nil
}

// selfSubject describes the caller as a report subject, recognising the
// ServiceAccount identity format so a pod's own report is labelled correctly.
func selfSubject(caller user.Info) v1alpha1.SubjectReference {
	name := caller.GetName()

	if rest, ok := strings.CutPrefix(name, serviceAccountUserPrefix); ok {
		if namespace, saName, found := strings.Cut(rest, ":"); found {
			return v1alpha1.SubjectReference{
				Kind:      v1alpha1.SubjectKindServiceAccount,
				Name:      saName,
				Namespace: namespace,
			}
		}
	}

	return v1alpha1.SubjectReference{Kind: v1alpha1.SubjectKindUser, Name: name}
}

// authorizeNonSelfReport gates reports about somebody else. Such a report
// discloses the full permission map of another subject, so it is a separate,
// explicitly granted capability - mirroring BulkSubjectAccessReview.
func (s *SubjectAccessStorage) authorizeNonSelfReport(ctx context.Context, caller user.Info) error {
	if s.authorizer == nil {
		return apierrors.NewInternalError(errors.New("authorizer is not configured"))
	}

	attrs := &accessAttributes{
		user: caller,
		resourceAttributes: &v1alpha1.ResourceAttributes{
			Verb:        "create",
			Group:       v1alpha1.GroupName,
			Version:     v1alpha1.SchemeGroupVersion.Version,
			Resource:    subjectAccessResource,
			Subresource: nonSelfReviewSubresource,
		},
	}

	decision, reason, err := s.authorizer.Authorize(ctx, attrs)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("authorize non-self SubjectAccessReport: %w", err))
	}
	if decision == authorizer.DecisionAllow {
		return nil
	}
	if reason == "" {
		reason = "reporting on another subject is not allowed"
	}

	return apierrors.NewForbidden(
		v1alpha1.Resource(subjectAccessResource+"/"+nonSelfReviewSubresource),
		"",
		errors.New(reason),
	)
}

func validateSubjectAccessSpec(spec *v1alpha1.SubjectAccessReportSpec) error {
	if len(spec.Namespaces) > maxRequestedNamespaces {
		return apierrors.NewBadRequest(fmt.Sprintf("spec.namespaces must contain no more than %d items", maxRequestedNamespaces))
	}
	if len(spec.Groups) > maxRequestedGroups {
		return apierrors.NewBadRequest(fmt.Sprintf("spec.groups must contain no more than %d items", maxRequestedGroups))
	}

	if spec.Subject == nil {
		return nil
	}

	if spec.Subject.Name == "" {
		return apierrors.NewBadRequest("spec.subject.name must not be empty")
	}

	switch spec.Subject.Kind {
	case v1alpha1.SubjectKindUser, v1alpha1.SubjectKindGroup:
		if spec.Subject.Namespace != "" {
			return apierrors.NewBadRequest("spec.subject.namespace is only allowed for ServiceAccount subjects")
		}
	case v1alpha1.SubjectKindServiceAccount:
		if spec.Subject.Namespace == "" {
			return apierrors.NewBadRequest("spec.subject.namespace is required for ServiceAccount subjects")
		}
	default:
		return apierrors.NewBadRequest(fmt.Sprintf("spec.subject.kind must be one of %s, %s, %s",
			v1alpha1.SubjectKindUser, v1alpha1.SubjectKindGroup, v1alpha1.SubjectKindServiceAccount))
	}

	return nil
}
