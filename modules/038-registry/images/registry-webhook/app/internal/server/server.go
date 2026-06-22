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

// Package server runs the registry-webhook's TLS HTTP admission server.
package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"registry-webhook/internal/mutate"
)

// Handler returns an http.Handler serving:
//
//	POST /mutate  – AdmissionReview v1 mutating webhook
//	GET  /healthz – liveness probe (200 OK)
func Handler(local mutate.Local) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /mutate", admitHandler(local))
	return mux
}

func admitHandler(local mutate.Local) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
			return
		}

		var review admissionv1.AdmissionReview
		if err := json.Unmarshal(body, &review); err != nil {
			http.Error(w, fmt.Sprintf("unmarshal AdmissionReview: %v", err), http.StatusBadRequest)
			return
		}

		req := review.Request
		resp := &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
		}

		var obj mutate.ModuleSource
		if err := json.Unmarshal(req.Object.Raw, &obj); err != nil {
			// Log but still allow — never block on a parse error.
			slog.Error("unmarshal ModuleSource", "err", err, "uid", req.UID)
		} else {
			ops, err := mutate.Mutate(obj, local)
			if err != nil {
				slog.Error("mutate", "err", err, "uid", req.UID, "name", obj.Metadata.Name)
			} else if len(ops) > 0 {
				patchBytes, err := json.Marshal(ops)
				if err != nil {
					slog.Error("marshal patch", "err", err, "uid", req.UID)
				} else {
					pt := admissionv1.PatchTypeJSONPatch
					resp.Patch = patchBytes
					resp.PatchType = &pt
				}
			}
		}

		out, err := json.Marshal(admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: resp,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}
}

// ListenAndServeTLS starts a TLS HTTP server on addr, loading the cert/key
// from certDir per handshake (hot-rotation safe). It blocks until the server
// exits.
func ListenAndServeTLS(addr, certDir string, h http.Handler) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: h,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: loadCertFromDir(certDir),
		},
	}
	return srv.ListenAndServeTLS("", "")
}

// loadCertFromDir returns a GetCertificate function that reads tls.crt and
// tls.key from dir on every TLS handshake, picking up rotated certificates
// without a restart.
func loadCertFromDir(dir string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(
			filepath.Join(dir, "tls.crt"),
			filepath.Join(dir, "tls.key"),
		)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}
}
