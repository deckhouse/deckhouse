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

package v1alpha2

import (
	"context"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var sshcredentialslog = logf.Log.WithName("sshcredentials-resource")

func (r *SSHCredentials) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

///+kubebuilder:webhook:path=/mutate-deckhouse-io-v1alpha2-sshcredentials,mutating=true,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=sshcredentials,verbs=create;update,versions=v1alpha2,name=msshcredentials.deckhouse.io,admissionReviewVersions=v1

type SSHCredentialsCustomDefaulter struct {}

var _ webhook.CustomDefaulter = &SSHCredentialsCustomDefaulter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *SSHCredentialsCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	sshCredentials, ok := obj.(*SSHCredentials)
	if !ok {
		return fmt.Errorf("expected an SSHCredentials object but got %T", obj)
	}
	sshcredentialslog.Info("default", "name", sshCredentials.GetName())

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-deckhouse-io-v1alpha2-sshcredentials,mutating=false,failurePolicy=fail,sideEffects=None,groups=deckhouse.io,resources=sshcredentials,verbs=create;update,versions=v1alpha2,name=vsshcredentials.deckhouse.io,admissionReviewVersions=v1

type SSHCredentialsCustomValidator struct {}

var _ webhook.CustomValidator = &SSHCredentialsCustomValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SSHCredentialsCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	sshCredentials, ok := obj.(*SSHCredentials)
	if !ok {
		return nil, fmt.Errorf("expected an SSHCredentials object but got %T", obj)
	}
	sshcredentialslog.Info("validate create", "name", sshCredentials.GetName())

	if len(sshCredentials.Spec.PrivateSSHKey) > 0 {
		privateSSHKey, err := base64.StdEncoding.DecodeString(sshCredentials.Spec.PrivateSSHKey)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid base64 encoded string")
		}

		_, err = ssh.ParseRawPrivateKey(privateSSHKey)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid private key encoded as base64 string")
		}
	}

	if sshCredentials.Spec.SudoPasswordEncoded != "" {
		_, err := base64.StdEncoding.DecodeString(sshCredentials.Spec.SudoPasswordEncoded)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "SudoPasswordEncoded"), "******", "SudoPasswordEncoded must be a valid base64 encoded string")
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SSHCredentialsCustomValidator) ValidateUpdate(_ context.Context, old, new runtime.Object) (admission.Warnings, error) {
	sshCredentials, ok := new.(*SSHCredentials)
	if !ok {
		return nil, fmt.Errorf("expected an SSHCredentials object but got %T", new)
	}
	sshcredentialslog.Info("validate update", "name", sshCredentials.GetName())

	if len(sshCredentials.Spec.PrivateSSHKey) > 0 {
		privateSSHKey, err := base64.StdEncoding.DecodeString(sshCredentials.Spec.PrivateSSHKey)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid base64 encoded string")
		}

		_, err = ssh.ParseRawPrivateKey(privateSSHKey)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "privateSSHKey"), "******", "privateSSHKey must be a valid private key encoded as base64 string")
		}
	}

	if sshCredentials.Spec.SudoPasswordEncoded != "" {
		_, err := base64.StdEncoding.DecodeString(sshCredentials.Spec.SudoPasswordEncoded)
		if err != nil {
			return nil, field.Invalid(field.NewPath("spec", "SudoPasswordEncoded"), "******", "SudoPasswordEncoded must be a valid base64 encoded string")
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SSHCredentialsCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	sshCredentials, ok := obj.(*SSHCredentials)
	if !ok {
		return nil, fmt.Errorf("expected an SSHCredentials object but got %T", obj)
	}
	sshcredentialslog.Info("validate delete", "name", sshCredentials.GetName())

	return nil, nil
}
