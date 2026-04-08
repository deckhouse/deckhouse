/*
Copyright 2025 Flant JSC

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

package csr

import (
	"context"
	"fmt"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

func init() {
	dynr.RegisterReconciler(rcname.CSRApprover, &certificatesv1.CertificateSigningRequest{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler automatically approves kubelet CertificateSigningRequests.
// It validates the CSR content according to Kubernetes kubelet serving
// certificate conventions and approves valid requests.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{csrNeedsApprovalPredicate()}
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Get the CSR.
	csr := &certificatesv1.CertificateSigningRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, csr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get csr %s: %w", req.Name, err)
	}

	// 2. Parse and validate the CSR.
	x509cr, err := parseCSR(csr)
	if err != nil {
		log.Info("failed to parse CSR, skipping", "csr", req.Name, "error", err.Error())
		return ctrl.Result{}, nil
	}

	// 3. Validate kubelet serving certificate if the signer is kubelet-serving.
	if csr.Spec.SignerName == "kubernetes.io/kubelet-serving" {
		if err := validateNodeServingCert(csr, x509cr); err != nil {
			log.Info("CSR validation failed, skipping", "csr", req.Name, "error", err.Error())
			return ctrl.Result{}, nil
		}
	}

	// 4. Approve the CSR.
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:           certificatesv1.CertificateApproved,
		Status:         corev1.ConditionTrue,
		Reason:         "AutoApproved",
		Message:        "autoapproved by node-controller/csr-approver",
		LastUpdateTime: metav1.Now(),
	})

	if err := r.Client.SubResource("approval").Update(ctx, csr); err != nil {
		return ctrl.Result{}, fmt.Errorf("approve csr %s: %w", req.Name, err)
	}

	log.Info("approved CSR", "csr", req.Name)
	return ctrl.Result{}, nil
}

// csrNeedsApprovalPredicate filters CSR events to only process CSRs that have
// not yet been approved, denied, or had a certificate issued.
func csrNeedsApprovalPredicate() predicate.Predicate {
	needsApproval := func(obj client.Object) bool {
		csr, ok := obj.(*certificatesv1.CertificateSigningRequest)
		if !ok {
			return false
		}
		if len(csr.Status.Certificate) != 0 {
			return false
		}
		return !isAlreadyApprovedOrDenied(csr)
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return needsApproval(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return needsApproval(e.ObjectNew)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}
