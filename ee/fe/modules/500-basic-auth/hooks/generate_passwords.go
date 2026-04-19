/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Set locations from config values or set a default one with generated password.

const (
	secretNS              = "kube-basic-auth"
	secretName            = "htpasswd"
	secretBinding         = "htpasswd_secret"
	locationsKey          = "basicAuth.locations"
	locationsInternalKey  = "basicAuth.internal.locations"
	generatedPasswdLength = 20
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       secretBinding,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{secretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{secretNS},
				},
			},
			// Synchronization is redundant because of OnBeforeHelm.
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   filterHtpasswdSecret,
		},
	},

	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, generatePassword)

func filterHtpasswdSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret to struct: %v", err)
	}

	return secret.Data, nil
}

const defaultUserName = `admin`

func generateDefaultLocation(password string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"users": map[string]interface{}{
				defaultUserName: password,
			},
			"location": "/",
		},
	}
}

func generatePassword(_ context.Context, input *go_hook.HookInput) error {
	// Set values from user controlled configuration.
	userLocations, ok := input.ConfigValues.GetOk(locationsKey)
	if ok {
		input.Values.Set(locationsInternalKey, userLocations.Value())
		return nil
	}

	// No config values. Try to restore generated password from the Secret.
	// Generate default location if no valid generated password available.

	pass, err := restorePasswordFromSnapshot(input.Snapshots.Get(secretBinding))
	if err != nil {
		input.Logger.Info("Generate default location for basic auth", log.Err(err))
		pass = pwgen.AlphaNum(generatedPasswdLength)
	}

	locations := generateDefaultLocation(pass)
	input.Values.Set(locationsInternalKey, locations)
	return nil
}

// restorePasswordFromSnapshot returns generated password for default location from Secret.
// password is considered generated if:
// - there is only 1 snapshot
// - there is only htpasswd field in Secret
// - there is only one line contains "admin:{PLAIN}" in htpasswd
//
// Hook should generate new default location if Secret
// contains more fields or passwords.
//
// NOTE: This algorithm is coupled with the field name in secret.yaml and "users" template in _helpers.tpl.
func restorePasswordFromSnapshot(snapshot []sdkpkg.Snapshot) (string, error) {
	// Only one Secret is expected.
	if len(snapshot) != 1 {
		return "", fmt.Errorf("secret/%s not found", secretName)
	}

	secretData := make(map[string][]byte, 0)

	err := snapshot[0].UnmarshalTo(&secretData)
	if err != nil {
		return "", fmt.Errorf("unmarshal to: %w", err)
	}

	// Expect only one user-password pair.
	if len(secretData) > 1 {
		return "", fmt.Errorf("secret/%s has multiple fields, possibly custom locations", secretName)
	}
	htpasswdBytes, ok := secretData["htpasswd"]
	if !ok {
		return "", fmt.Errorf("'htpasswd' field is missing in secret/%s", secretName)
	}
	htpasswd := string(htpasswdBytes)

	if strings.Count(htpasswd, "{PLAIN}") != 1 {
		return "", fmt.Errorf("secret/%s has many users in htpasswd field", secretName)
	}

	userPrefix := defaultUserName + ":{PLAIN}"
	if strings.Count(htpasswd, userPrefix) != 1 {
		return "", fmt.Errorf("secret/%s has no password for %s user", secretName, defaultUserName)
	}

	// Extract password.
	cleaned := strings.TrimSpace(htpasswd)
	pass := strings.TrimPrefix(cleaned, userPrefix)

	return pass, nil
}
