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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
	"github.com/deckhouse/deckhouse/pkg/log"
)

/*
This hook is similar to the hook from go_lib/hooks/generate_password.
The difference is that this hook handles passwords for 2 apps
at once: for the webui and for the status.
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/upmeter/generate_password",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       authSecretBinding,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: authSecretNames,
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{upmeterNS},
				},
			},
			// Synchronization is redundant because of OnBeforeHelm.
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   filterAuthSecret,
		},
	},

	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, restoreOrGeneratePassword)

const (
	upmeterNS         = "d8-upmeter"
	authSecretField   = "auth"
	authSecretBinding = "auth-secrets"
	statusSecretName  = "basic-auth-status"
	webuiSecretName   = "basic-auth-webui"

	externalAuthValuesTmpl     = "upmeter.auth.%s.externalAuthentication"
	passwordInternalValuesTmpl = "upmeter.internal.auth.%s.password"

	generatedPasswdLength = 20
)

var authSecretNames = []string{statusSecretName, webuiSecretName}
var upmeterApps = map[string]string{
	statusSecretName: "status",
	webuiSecretName:  "webui",
}

type storedPassword struct {
	SecretName string            `json:"name"`
	Data       map[string][]byte `json:"data"`
}

func filterAuthSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to struct: %v", err)
	}

	return storedPassword{
		SecretName: secret.GetName(),
		Data:       secret.Data,
	}, nil
}

// restoreOrGeneratePassword restores passwords from config values or secrets.
// If there are no passwords, it generates new.
func restoreOrGeneratePassword(_ context.Context, input *go_hook.HookInput) error {
	for secretName, appName := range upmeterApps {
		externalAuthValuesPath := fmt.Sprintf(externalAuthValuesTmpl, appName)
		passwordInternalValuesPath := fmt.Sprintf(passwordInternalValuesTmpl, appName)

		// Clear password from internal values if an external authentication is enabled.
		if input.Values.Exists(externalAuthValuesPath) {
			input.Values.Remove(passwordInternalValuesPath)
			continue
		}

		// Try to restore generated password from the Secret, or generate a new one.
		pass, err := restoreGeneratedPasswordFromSnapshot(input.Snapshots.Get(authSecretBinding), secretName)
		if err != nil {
			input.Logger.Info("No password in config values, generate new one", slog.String("name", appName), log.Err(err))
			pass = GeneratePassword()
		}

		input.Values.Set(passwordInternalValuesPath, pass)
	}

	return nil
}

func GeneratePassword() string {
	return pwgen.AlphaNum(generatedPasswdLength)
}

// restoreGeneratedPasswordFromSnapshot extracts password from the plain basic auth string:
// admin:{PLAIN}password
func restoreGeneratedPasswordFromSnapshot(snapshot []pkg.Snapshot, secretName string) (string, error) {
	var secretData map[string][]byte
	var hasSecret = false
	// Find snapshot for appName.
	for storedPassword, err := range sdkobjectpatch.SnapshotIter[storedPassword](snapshot) {
		if err != nil {
			return "", fmt.Errorf("failed to iterate over snapshots: %w", err)
		}

		if storedPassword.SecretName == secretName {
			secretData = storedPassword.Data
			hasSecret = true
		}
	}

	if !hasSecret {
		return "", fmt.Errorf("secret/%s not found", secretName)
	}

	// Expect one field with basic auth.
	if secretData == nil {
		return "", fmt.Errorf("secret/%s has empty data", secretName)
	}
	if len(secretData) != 1 {
		return "", fmt.Errorf("secret/%s has more than one field", secretName)
	}
	authBytes, ok := secretData[authSecretField]
	if !ok {
		return "", fmt.Errorf("secret/%s has no %s field", secretName, authSecretField)
	}

	// Extract password from basic auth.
	auth := strings.TrimSpace(string(authBytes))
	parts := strings.SplitN(auth, "{PLAIN}", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("secret/%s has %s field with malformed basic auth plain password", secretName, authSecretField)
	}
	pass := strings.TrimSpace(parts[1])

	return pass, nil
}
