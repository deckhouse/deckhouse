/*
Copyright 2021 Flant JSC

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

package generate_password

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	secretBindingName          = "password_secret"
	defaultBasicAuthPlainField = "auth"
	defaultBeforeHelmOrder     = 10
	generatedPasswdLength      = 20
)

func NewBasicAuthPlainHook(settings HookSettings) *Hook {
	// Ensure camelCase for moduleValuesPath
	valuesKey := addonutils.ModuleNameToValuesKey(settings.ModuleName)
	return &Hook{
		Secret: Secret{
			Namespace: settings.Namespace,
			Name:      settings.SecretName,
		},
		ValuesKey: valuesKey,
	}
}

type HookSettings struct {
	ModuleName string
	Namespace  string
	SecretName string
}

// RegisterHook returns func to register common hook that generates
// and stores a password in the Secret.
// if ExternalAuth is used - secret will be deleted, you can change this behavior by `keepPasswordOnExternalAuth` flag
func RegisterHook(settings HookSettings) bool {
	hook := NewBasicAuthPlainHook(settings)
	return sdk.RegisterFunc(&go_hook.HookConfig{
		Queue: fmt.Sprintf("/modules/%s/generate_password", hook.ValuesKey),
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       secretBindingName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{hook.Secret.Name},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{hook.Secret.Namespace},
					},
				},
				// Synchronization is redundant because of OnBeforeHelm.
				ExecuteHookOnSynchronization: go_hook.Bool(false),
				FilterFunc:                   hook.Filter,
			},
		},
		OnBeforeHelm: &go_hook.OrderedConfig{Order: float64(defaultBeforeHelmOrder)},
	}, hook.Handle)
}

type Hook struct {
	Secret                     Secret
	ValuesKey                  string
	keepPasswordOnExternalAuth bool
}

type Secret struct {
	Namespace string
	Name      string
}

// Filter extracts password from the Secret. Password can be stored as a raw string or as
// a basic auth plain format (user:{PLAIN}password). Custom FilterFunc is called for custom
// password extraction.
func (h *Hook) Filter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to struct: %v", err)
	}

	return secret.Data, nil
}

// Handle restores password from the configuration or from the Secret and
// puts it to internal values.
// It generates new password if there is no password in the configuration
// and no Secret found.
func (h *Hook) Handle(_ context.Context, input *go_hook.HookInput) error {
	externalAuthKey := h.ExternalAuthKey()
	passwordInternalKey := h.PasswordInternalKey()

	// Clear password from internal values if an external authentication is enabled.
	if input.Values.Exists(externalAuthKey) && !h.keepPasswordOnExternalAuth {
		input.Values.Remove(passwordInternalKey)
		return nil
	}

	// Try to restore generated password from the Secret, or generate a new one.
	pass, err := h.restoreGeneratedPasswordFromSnapshot(input.Snapshots.Get(secretBindingName))
	if err != nil {
		input.Logger.Info("No password in Secret, generate new one", log.Err(err))
		pass = GeneratePassword()
	}

	input.Values.Set(passwordInternalKey, pass)
	return nil
}

const (
	externalAuthKeyTmpl     = "%s.auth.externalAuthentication"
	passwordKeyTmpl         = "%s.auth.password"
	passwordInternalKeyTmpl = "%s.internal.auth.password"
)

func (h *Hook) ExternalAuthKey() string {
	return fmt.Sprintf(externalAuthKeyTmpl, h.ValuesKey)
}

func (h *Hook) PasswordKey() string {
	return fmt.Sprintf(passwordKeyTmpl, h.ValuesKey)
}

func (h *Hook) PasswordInternalKey() string {
	return fmt.Sprintf(passwordInternalKeyTmpl, h.ValuesKey)
}

// restoreGeneratedPasswordFromSnapshot extracts password from the plain basic auth string:
// username:{PLAIN}password
func (h *Hook) restoreGeneratedPasswordFromSnapshot(snapshots []sdkpkg.Snapshot) (string, error) {
	if len(snapshots) != 1 {
		return "", fmt.Errorf("secret/%s not found", h.Secret.Name)
	}

	secretData := make(map[string][]byte, 0)
	// Expect one field with basic auth
	err := snapshots[0].UnmarshalTo(&secretData)
	if err != nil {
		return "", fmt.Errorf("secret/%s has empty data: %w", h.Secret.Name, err)
	}

	if len(secretData) != 1 {
		return "", fmt.Errorf("secret/%s has more than one field", h.Secret.Name)
	}

	authBytes, ok := secretData[defaultBasicAuthPlainField]
	if !ok {
		return "", fmt.Errorf("secret/%s has no %s field", h.Secret.Name, defaultBasicAuthPlainField)
	}

	// Extract password from basic auth.
	auth := strings.TrimSpace(string(authBytes))
	parts := strings.SplitN(auth, "{PLAIN}", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("secret/%s has %s field with malformed basic auth plain password", h.Secret.Name, defaultBasicAuthPlainField)
	}
	pass := strings.TrimSpace(parts[1])

	return pass, nil
}

func GeneratePassword() string {
	return pwgen.AlphaNum(generatedPasswdLength)
}
