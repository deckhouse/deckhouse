package hooks

import (
	"fmt"

	"github.com/chr4/pwgen"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
)

type DexAuthenticator struct {
	ID          string                 `json:"uuid"`
	EncodedName string                 `json:"encodedName"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Spec        map[string]interface{} `json:"spec"`

	AllowAccessToKubernetes bool        `json:"allowAccessToKubernetes"`
	Credentials             Credentials `json:"credentials"`
}

type Credentials struct {
	CookieSecret string `json:"cookieSecret"`
	AppDexSecret string `json:"appDexSecret"`
}

type DexAuthenticatorSecret struct {
	ID          string      `json:"uuid"`
	Name        string      `json:"name"`
	Namespace   string      `json:"namespace"`
	Credentials Credentials `json:"credentials"`
}

func applyDexAuthenticatorFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from dex authenticator: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("dex authenticator has no spec field")
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()

	id := fmt.Sprintf("%s@%s", name, namespace)
	encodedName := encoding.ToFnvLikeDex(fmt.Sprintf("%s-%s-dex-authenticator", name, namespace))

	_, allowAccessToKubernetes := obj.GetAnnotations()["dexauthenticator.deckhouse.io/allow-access-to-kubernetes"]
	if namespace != "d8-dashboard" {
		allowAccessToKubernetes = false
	}

	return DexAuthenticator{
		ID:                      id,
		EncodedName:             encodedName,
		Name:                    name,
		Namespace:               namespace,
		Spec:                    spec,
		AllowAccessToKubernetes: allowAccessToKubernetes,
	}, nil
}

func applyDexAuthenticatorSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert dex authenticator secret to secret: %v", err)
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()

	id := fmt.Sprintf("%s@%s", name, namespace)
	return DexAuthenticatorSecret{
		ID:        id,
		Name:      name,
		Namespace: namespace,
		Credentials: Credentials{
			AppDexSecret: string(secret.Data["client-secret"]),
			CookieSecret: string(secret.Data["cookie-secret"]),
		},
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "authenticators",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "DexAuthenticator",
			FilterFunc: applyDexAuthenticatorFilter,
		},
		{
			Name:       "credentials",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "dex-authenticator",
					"name": "credentials",
				},
			},
			FilterFunc: applyDexAuthenticatorSecretFilter,
		},
	},
}, getDexAuthenticator)

func getDexAuthenticator(input *go_hook.HookInput) error {
	authenticators := input.Snapshots["authenticators"]
	credentials := input.Snapshots["credentials"]

	credentialsByID := make(map[string]Credentials, len(credentials))

	for _, secret := range credentials {
		dexSecret, ok := secret.(DexAuthenticatorSecret)
		if !ok {
			return fmt.Errorf("cannot convert dex authenticator secret")
		}

		credentialsByID[dexSecret.ID] = dexSecret.Credentials
	}

	dexAuthenticators := make([]DexAuthenticator, 0, len(authenticators))
	for _, authenticator := range authenticators {
		dexAuthenticator, ok := authenticator.(DexAuthenticator)
		if !ok {
			return fmt.Errorf("cannot convert dex authenticaor")
		}

		existedCredentials, ok := credentialsByID[fmt.Sprintf("dex-authenticator-%s", dexAuthenticator.ID)]
		if !ok {
			existedCredentials = Credentials{
				AppDexSecret: pwgen.AlphaNum(20),
				CookieSecret: pwgen.AlphaNum(20),
			}
		}

		dexAuthenticator.Credentials = existedCredentials
		dexAuthenticators = append(dexAuthenticators, dexAuthenticator)
	}

	input.Values.Set("userAuthn.internal.dexAuthenticatorCRDs", dexAuthenticators)
	return nil
}
