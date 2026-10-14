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

package controlplaneoperation

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

const apiServerPort = "6443"

// readyzClient probes the local kube-apiserver like kubelet does: anonymous and without
// verifying the serving certificate. Only the status code is read, and a CA mismatch during
// rotation must not block the operation forever.
var readyzClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // see above
	},
}

// apiServerReadyz checks /readyz of this node's kube-apiserver; other components have nothing to
// check.
func (r *Reconciler) apiServerReadyz(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation) error {
	if op.Spec.Component != controlplanev1alpha1.OperationComponentKubeAPIServer {
		return nil
	}
	return checkReadyz(ctx, readyzClient, "https://"+net.JoinHostPort(r.node.AdvertiseIP, apiServerPort)+"/readyz")
}

// checkReadyz requires GET url to return 200. The kube-apiserver static pod readiness probe is
// /healthz, which turns green before /readyz (informer sync, post-start hooks), so a Ready pod
// can still refuse traffic.
func checkReadyz(ctx context.Context, httpClient *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned %d", url, resp.StatusCode)
	}
	return nil
}
