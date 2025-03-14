/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"net/http"

	"github.com/go-chi/render"
)

// ErrResponse renderer type for handling all sorts of errors.
//
// In the best case scenario, the excellent github.com/pkg/errors package
// helps reveal information on the error, setting it on Err, and in the Render()
// method, using it to set the application-specific error code in AppCode.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrStatus(err error, code int) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: code,
		StatusText:     http.StatusText(code),
		ErrorText:      err.Error(),
	}
}

func ErrText(text string, code int) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: code,
		StatusText:     http.StatusText(code),
		ErrorText:      text,
	}
}

func ErrInternalError(err error) render.Renderer {
	return ErrStatus(err, http.StatusInternalServerError)
}

func ErrInternalErrorText(text string) render.Renderer {
	return ErrText(text, http.StatusInternalServerError)
}

func ErrBadRequest(err error) render.Renderer {
	return ErrStatus(err, http.StatusBadRequest)
}

// ChangesReponse represents a model to track applied changes
type ChangesReponse struct {
	Distribution bool `json:",omitempty"` // Indicates changes in the distribution configuration.
	Auth         bool `json:",omitempty"` // Indicates changes in the authentication system.
	PKI          bool `json:",omitempty"` // Indicates changes in the public key infrastructure.
	Pod          bool `json:",omitempty"` // Indicates changes in the pod setup.
	Mirrorer     bool `json:",omitempty"` // Indicates changes in the mirrorer configuration.
}

// HasChanges checks if any field in ChangesModel is true.
func (c ChangesReponse) HasChanges() bool {
	return c.Distribution || c.Auth || c.PKI || c.Pod || c.Mirrorer
}

func (c ChangesReponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
