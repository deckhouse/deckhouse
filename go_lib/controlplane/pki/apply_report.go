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

package pki

// PKIApplyReport describes outcomes of CreatePKIBundle for each logical artifact
// (root CA, leaf certificate pair, or service account key pair). Entry order follows
// the order in which artifacts are processed and is not guaranteed to be stable
// across releases because it depends on map iteration.
type PKIApplyReport struct {
	Entries []PKIApplyEntry
}

// PKIApplyEntry is one row in PKIApplyReport. Name is the file base path relative
// to the PKI directory (e.g. "ca", "etcd/server", "sa" for service account keys).
type PKIApplyEntry struct {
	Name   string
	Kind   PKIEntryKind
	Action PKIEntryAction
}

// PKIEntryKind classifies the artifact reported by CreatePKIBundle.
type PKIEntryKind uint8

const (
	// PKIEntryKindRootCA is a self-signed CA certificate (e.g. ca, etcd/ca).
	PKIEntryKindRootCA PKIEntryKind = iota
	// PKIEntryKindLeafCert is a leaf certificate signed by a CA.
	PKIEntryKindLeafCert
	// PKIEntryKindServiceAccountKeys is the sa.key / sa.pub pair (not X.509).
	PKIEntryKindServiceAccountKeys
)

// PKIEntryAction describes what happened to the artifact on disk.
type PKIEntryAction uint8

const (
	// PKIActionUnchanged means existing material was kept (validation passed or SA pair complete).
	PKIActionUnchanged PKIEntryAction = iota
	// PKIActionWrittenCreated means new key material was written and there was no usable prior cert/key.
	PKIActionWrittenCreated
	// PKIActionWrittenRegenerated means existing leaf material was replaced (validation failed, read error, etc.).
	PKIActionWrittenRegenerated
	// PKIActionSAPublicKeyRestored means sa.key existed and only sa.pub was written.
	PKIActionSAPublicKeyRestored
)

func (r *PKIApplyReport) add(name string, kind PKIEntryKind, action PKIEntryAction) {
	r.Entries = append(r.Entries, PKIApplyEntry{
		Name:   name,
		Kind:   kind,
		Action: action,
	})
}
