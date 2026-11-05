/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package scope is the receiver side of scoped reads of aggregated APIs in
// Deckhouse: kube-apiserver computes the namespaces a scoped read may see and
// sends them to the extension server in the trusted headers
// X-Deckhouse-Scope-Mode and X-Deckhouse-Scope-Namespaces.
//
// The package has three layers:
//
//   - Trust. WithTrusted (k8s.io/apiserver based servers) and
//     NewTrustedHandler (plain net/http servers) decide whether a request came
//     from kube-apiserver's front proxy. Only then are the headers parsed into
//     a Decision and stored in the request context. On every request the
//     headers are deleted afterwards, so nothing further down can read them.
//   - Authorization. WithAuthorizer wraps the delegated authorizer: a read of
//     an opted-in resource that carries a filtered Decision is allowed without
//     the delegated SubjectAccessReview; the storage restricts the data.
//   - Filtering. Allowed, FilterList, WatchFilter and CheckGet restrict what
//     the storage serves by the Decision in the context.
//
// FromContext is the only source of the Decision: no other code reads the
// headers. The transport of the decision (headers today, possibly a decision
// handle resolved through an API later) stays internal to this package.
package scope
