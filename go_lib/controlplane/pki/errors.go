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

import (
	"fmt"
	"time"
)

// CertValidationError is returned by createRootCertIfNotExists when an existing CA
// certificate does not satisfy the current configuration. The caller should treat
// this as a hard error — CA certificates are never auto-regenerated.
type CertValidationError struct {
	BaseName string
	Reason   string
}

func (e *CertValidationError) Error() string {
	return fmt.Sprintf("certificate %q are not valid: %s", e.BaseName, e.Reason)
}

// MissingError is returned when a certificate file does not exist on disk.
// The caller treats this as a skippable condition.
type MissingError struct {
	BaseName string
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("certificate %q not found on disk", e.BaseName)
}

// CAMissingError is returned when the signing CA certificate file is absent.
// The leaf cannot be re-signed, so renewal is skipped.
type CAMissingError struct {
	CAName string
}

func (e *CAMissingError) Error() string {
	return fmt.Sprintf("CA %q certificate not found on disk", e.CAName)
}

// CAExternalError is returned when the CA certificate exists but its private key does not (external CA scenario).
// Renewal is skipped.
type CAExternalError struct {
	CAName string
}

func (e *CAExternalError) Error() string {
	return fmt.Sprintf("CA %q private key not found (external CA — renewal skipped)", e.CAName)
}

// CAExpiredError is returned when the signing CA certificate has already expired.
// Renewal is skipped: a leaf signed by an expired CA fails chain validation, so re-signing is pointless until the CA is rotated.
type CAExpiredError struct {
	CAName    string
	ExpiredAt time.Time
}

func (e *CAExpiredError) Error() string {
	return fmt.Sprintf("CA %q expired at %s — renewing leaf certs against an expired CA is pointless; rotate the CA first",
		e.CAName, e.ExpiredAt.UTC().Format(time.RFC3339),
	)
}
