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

package kubeconfig

import (
	"fmt"
	"time"
)

// MissingError is returned when a kubeconfig file does not exist on disk.
// The caller treats this as a skippable condition.
type MissingError struct {
	File File
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("kubeconfig file %q not found", e.File)
}

// CAMissingError is returned when the signing CA certificate file is absent.
// The client cert cannot be re-signed, so renewal is skipped.
type CAMissingError struct {
	CAName string
}

func (e *CAMissingError) Error() string {
	return fmt.Sprintf("CA %q certificate not found on disk", e.CAName)
}

// CAExternalError is returned when the CA certificate exists but its private key does not (external CA scenario).
// Renewal of the kubeconfig client cert is skipped — only the holder of the CA key can sign new leaf certificates.
type CAExternalError struct {
	CAName string
}

func (e *CAExternalError) Error() string {
	return fmt.Sprintf("CA %q private key not found (external CA — renewal skipped)", e.CAName)
}

// CAExpiredError is returned when the CA certificate has already expired.
// Renewal is skipped: a client cert signed by an expired CA fails chain validation, so re-signing is pointless until the CA is rotated.
type CAExpiredError struct {
	CAName    string
	ExpiredAt time.Time
}

func (e *CAExpiredError) Error() string {
	return fmt.Sprintf(
		"CA %q expired at %s — renewing kubeconfig certs against an expired CA is pointless; rotate the CA first",
		e.CAName, e.ExpiredAt.UTC().Format(time.RFC3339),
	)
}
