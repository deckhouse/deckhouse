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

package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	// Repo && Store
	ErrUnknownRepository = errors.New("unknown repository")
	ErrManifestNotFound  = errors.New("manifest not found")
	ErrBlobNotFound      = errors.New("blob not found")
	ErrInvalidDigest     = errors.New("invalid digest")
	ErrMissingReference  = errors.New("missing reference")
)

// mapStatusError Repo && Store errors to
func MapStatusError(err error) *ErrorStatus {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, ErrUnknownRepository):
		return ErrStatusRepoNameUnknown

	case errors.Is(err, ErrManifestNotFound):
		return ErrStatusManifestUnknown

	case errors.Is(err, ErrBlobNotFound):
		return ErrStatusBlobUnknown

	case errors.Is(err, ErrInvalidDigest):
		return ErrStatusDigestInvalid

	case errors.Is(err, ErrMissingReference):
		return ErrStatusMissingReference

	default:
		return ErrStatusInternalServerError.WithMessage(err.Error())
	}
}

// Registry error status
var (
	// Repo name
	ErrStatusRepoNameInvalid = &ErrorStatus{
		status:  http.StatusBadRequest,
		code:    "NAME_INVALID",
		message: "Invalid name",
	}

	ErrStatusRepoNameUnknown = &ErrorStatus{
		status:  http.StatusNotFound,
		code:    "NAME_UNKNOWN",
		message: "Unknown name",
	}

	// Digest
	ErrStatusDigestInvalid = &ErrorStatus{
		status:  http.StatusBadRequest,
		code:    "DIGEST_INVALID",
		message: "Invalid digest",
	}

	ErrStatusDigestNotSpecified = &ErrorStatus{
		status:  http.StatusBadRequest,
		code:    "DIGEST_INVALID",
		message: "Digest not specified",
	}

	// Blob
	ErrStatusBlobUnknown = &ErrorStatus{
		status:  http.StatusNotFound,
		code:    "BLOB_UNKNOWN",
		message: "Unknown blob",
	}

	ErrStatusBlobUnknownRange = &ErrorStatus{
		status:  http.StatusRequestedRangeNotSatisfiable,
		code:    "BLOB_UNKNOWN",
		message: "Range not satisfiable",
	}

	// Manifest
	ErrStatusManifestUnknown = &ErrorStatus{
		status:  http.StatusNotFound,
		code:    "MANIFEST_UNKNOWN",
		message: "Unknown manifest",
	}

	// Reference
	ErrStatusMissingReference = &ErrorStatus{
		status:  http.StatusBadRequest,
		code:    "BAD_REQUEST",
		message: "Missing reference",
	}

	// Global
	ErrStatusBadRequest = &ErrorStatus{
		status:  http.StatusBadRequest,
		code:    "BAD_REQUEST",
		message: "Bad request",
	}

	ErrStatusMethodUnknown = &ErrorStatus{
		status:  http.StatusBadRequest,
		code:    "METHOD_UNKNOWN",
		message: "We don't understand your method + url",
	}

	ErrStatusUnsupported = &ErrorStatus{
		status:  http.StatusMethodNotAllowed,
		code:    "UNSUPPORTED",
		message: "Unsupported operation",
	}

	ErrStatusInternalServerError = &ErrorStatus{
		status:  http.StatusInternalServerError,
		code:    "INTERNAL_SERVER_ERROR",
		message: "internal server error",
	}
)

// ErrorStatus represents a registry API error with HTTP status and distribution-spec code.
type ErrorStatus struct {
	status  int
	code    string
	message string
}

func (e *ErrorStatus) String() string {
	return fmt.Sprintf(
		"%d %s %s",
		e.status,
		e.code,
		e.message,
	)
}

// Write writes the error to the response writer as JSON.
func (e *ErrorStatus) Write(resp http.ResponseWriter) error {
	resp.WriteHeader(e.status)

	type err struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	type wrap struct {
		Errors []err `json:"errors"`
	}

	return json.NewEncoder(resp).Encode(wrap{
		Errors: []err{
			{
				Code:    e.code,
				Message: e.message,
			},
		},
	})
}

func (e *ErrorStatus) WithMessage(message string) *ErrorStatus {
	if e == nil {
		return nil
	}

	ret := e.copy()
	ret.message = message
	return ret
}

func (e *ErrorStatus) WithStatus(status int) *ErrorStatus {
	if e == nil {
		return nil
	}

	ret := e.copy()
	ret.status = status
	return ret
}

func (e *ErrorStatus) copy() *ErrorStatus {
	if e == nil {
		return nil
	}

	return &ErrorStatus{
		status:  e.status,
		code:    e.code,
		message: e.message,
	}
}
