/*
Copyright 2023.

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

package v1alpha1

import (
	"encoding/base64"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var staticinstancecredentialslog = logf.Log.WithName("staticinstancecredentials-resource")

func (r *StaticInstanceCredentials) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha1-staticinstancecredentials,mutating=true,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticinstancecredentials,verbs=create;update,versions=v1alpha1,name=mstaticinstancecredentials.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &StaticInstanceCredentials{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *StaticInstanceCredentials) Default() {
	staticinstancecredentialslog.Info("default", "name", r.Name)

	if r.Spec.SSHPort == 0 {
		r.Spec.SSHPort = 22
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1alpha1-staticinstancecredentials,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=staticinstancecredentials,verbs=create;update,versions=v1alpha1,name=vstaticinstancecredentials.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &StaticInstanceCredentials{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticInstanceCredentials) ValidateCreate() (admission.Warnings, error) {
	staticinstancecredentialslog.Info("validate create", "name", r.Name)

	privateSSHKey, err := base64.StdEncoding.DecodeString(r.Spec.PrivateSSHKey)
	if err != nil {
		return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid base64 encoded string")
	}

	_, err = ssh.ParseRawPrivateKey(privateSSHKey)
	if err != nil {
		return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid private key encoded as base64 string")
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *StaticInstanceCredentials) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	staticinstancecredentialslog.Info("validate update", "name", r.Name)

	privateSSHKey, err := base64.StdEncoding.DecodeString(r.Spec.PrivateSSHKey)
	if err != nil {
		return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid base64 encoded string")
	}

	_, err = ssh.ParseRawPrivateKey(privateSSHKey)
	if err != nil {
		return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid private key encoded as base64 string")
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *StaticInstanceCredentials) ValidateDelete() (admission.Warnings, error) {
	staticinstancecredentialslog.Info("validate delete", "name", r.Name)

	return nil, nil
}
