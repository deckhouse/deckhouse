// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package metrics provides centralized metric names of kube-rbac-auth and registers them.
// The middleware sets a metric only by these constants, so what it sets and what is registered
// cannot drift apart.
package metrics

import (
	"fmt"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

// Metric name constants organized by functional area. Every application that embeds
// the middleware exposes them under these names.
const (
	// ============================================================================
	// Request Metrics
	// ============================================================================

	// RequestsTotal counts the requests the middleware checked, by the result of the check
	// and the credential the result is about.
	RequestsTotal = "kube_rbac_auth_requests_total"

	// ============================================================================
	// Review Metrics
	// ============================================================================

	// ReviewRequestsTotal counts the TokenReview and SubjectAccessReview requests to the API server,
	// retries included.
	ReviewRequestsTotal = "kube_rbac_auth_review_requests_total"
	// StaleResultsTotal counts the review results used after their lifetime because the API server
	// could not answer.
	StaleResultsTotal = "kube_rbac_auth_stale_results_total"
)

// Results of a request check, the result label of RequestsTotal.
const (
	ResultAllowed      = "allowed"      // ResultAllowed: the request reached the application.
	ResultUnauthorized = "unauthorized" // ResultUnauthorized: no valid credential, 401.
	ResultForbidden    = "forbidden"    // ResultForbidden: the credential has no access, 403.
	ResultError        = "error"        // ResultError: the API server could not check it and nothing was cached, 500.
)

// Credentials a request check is about, the credential label of RequestsTotal.
const (
	CredentialCertificate = "certificate" // CredentialCertificate: a trusted client certificate.
	CredentialToken       = "token"       // CredentialToken: the bearer token, checked with a TokenReview.
	CredentialNone        = "none"        // CredentialNone: neither a token nor a trusted certificate.
)

// Reviews, the review label of ReviewRequestsTotal and StaleResultsTotal.
const (
	TokenReview         = "TokenReview"
	SubjectAccessReview = "SubjectAccessReview"
)

// Results of a request to the API server, the result label of ReviewRequestsTotal.
const (
	ReviewResultSuccess = "success" // ReviewResultSuccess: the API server answered.
	ReviewResultError   = "error"   // ReviewResultError: the request failed, with no answer to use.
)

// ============================================================================
// Metric Registration Functions
// ============================================================================

// RegisterKubeRBACAuthMetrics registers all metrics of the middleware with their HELP and labels.
// New calls it before the middleware sets any of them.
func RegisterKubeRBACAuthMetrics(metricStorage metricsstorage.Storage) error {
	if err := registerRequestMetrics(metricStorage); err != nil {
		return fmt.Errorf("register request metrics: %w", err)
	}

	if err := registerReviewMetrics(metricStorage); err != nil {
		return fmt.Errorf("register review metrics: %w", err)
	}

	return nil
}

// registerRequestMetrics registers the counter of the requests the middleware checked.
func registerRequestMetrics(metricStorage metricsstorage.Storage) error {
	_, err := metricStorage.RegisterCounter(
		RequestsTotal,
		[]string{LabelResult, LabelCredential},
		options.WithHelp("Counter of requests checked by kube-rbac-auth by result (allowed, unauthorized, forbidden, error) and by the credential the result is about (certificate, token, none)"),
	)
	if err != nil {
		return fmt.Errorf("failed to register %s: %w", RequestsTotal, err)
	}

	return nil
}

// registerReviewMetrics registers the counters of the reviews the middleware asks the API server for.
func registerReviewMetrics(metricStorage metricsstorage.Storage) error {
	counters := []struct {
		name   string
		labels []string
		help   string
	}{
		{ReviewRequestsTotal, []string{LabelReview, LabelResult}, "Counter of requests of kube-rbac-auth to the API server by review (TokenReview, SubjectAccessReview) and result (success, error), retries included"},
		{StaleResultsTotal, []string{LabelReview}, "Counter of TokenReview and SubjectAccessReview results that kube-rbac-auth used after their lifetime because the API server could not answer"},
	}

	for _, c := range counters {
		if _, err := metricStorage.RegisterCounter(c.name, c.labels, options.WithHelp(c.help)); err != nil {
			return fmt.Errorf("failed to register %s: %w", c.name, err)
		}
	}

	return nil
}
