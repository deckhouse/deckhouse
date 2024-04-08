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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "order_packages_proxy_token",
			Crontab: "23 * * * *",
		},
	},
	Queue: "/modules/node-manager/order_packages_proxy_token",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "packages_proxy_token",
			ApiVersion:          "v1",
			Kind:                "Secret",
			ExecuteHookOnEvents: pointer.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"packages-proxy-token"},
			},
			FilterFunc: packagesProxyTokenFilterSecret,
		},
	},
}, dependency.WithExternalDependencies(handleToken))

func packagesProxyTokenFilterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	var validFor time.Duration
	if sec.Type == corev1.SecretTypeOpaque {
		expireRaw, ok := sec.Data["expiration"]
		if ok {
			expire, err := time.Parse(time.RFC3339, string(expireRaw))
			if err != nil {
				return nil, err
			}

			validFor = time.Until(expire)
		}
	}

	return packagesProxyTokenSecret{
		ValidFor:           validFor,
		PackagesProxyToken: string(sec.Data["token"]),
	}, nil
}

type packagesProxyTokenSecret struct {
	ValidFor           time.Duration
	PackagesProxyToken string
}

func handleToken(input *go_hook.HookInput, dc dependency.Container) error {
	snap := input.Snapshots["packages_proxy_token"]
	if len(snap) > 1 {
		return fmt.Errorf("more than one secret found")
	}

	if len(snap) == 1 {
		token := snap[0].(packagesProxyTokenSecret)
		if token.ValidFor > time.Hour*24*14 {
			input.Values.Set("nodeManager.internal.packagesProxyToken", token.PackagesProxyToken)
			return nil
		}
	}

	PackagesProxyToken, err := generateNewToken(dc)
	if err != nil {
		return err
	}
	input.Values.Set("nodeManager.internal.packagesProxyToken", PackagesProxyToken)
	return nil
}

func generateNewToken(dc dependency.Container) (string, error) {
	var token string

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return token, err
	}

	// generate SA if not exists
	_, err = k8sClient.CoreV1().ServiceAccounts("d8-cloud-instance-manager").Get(context.TODO(), "packages-proxy-sa", v1.GetOptions{})
	if errors.IsNotFound(err) {
		sa := &corev1.ServiceAccount{
			TypeMeta: v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "packages-proxy-sa",
				Namespace: "d8-cloud-instance-manager",
				Labels: map[string]string{
					"heritage": "deckhouse",
					"module":   "node-manager",
				},
			},
		}
		_, err := k8sClient.CoreV1().ServiceAccounts("d8-cloud-instance-manager").Create(context.TODO(), sa, v1.CreateOptions{})
		if err != nil {
			return token, err
		}
	}

	// set token parameters
	expiration := int64(time.Hour * 24 * 365)
	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{"api"},
			ExpirationSeconds: &expiration,
		},
	}

	// generate token
	response, err := k8sClient.CoreV1().ServiceAccounts("d8-cloud-instance-manager").CreateToken(context.TODO(), "packages-proxy-sa", tokenRequest, v1.CreateOptions{})
	if err != nil {
		return token, err
	}
	if len(response.Status.Token) == 0 {
		return token, fmt.Errorf("failed to create token: no token in server response")
	}

	token = response.Status.Token
	secret := &corev1.Secret{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "packages-proxy-token",
			Namespace: "d8-cloud-instance-manager",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "node-manager",
			},
		},
		Data: map[string][]byte{
			"token":               []byte(token),
			"expirationTimestamp": []byte((response.Status.ExpirationTimestamp.String())),
		},
		Type: corev1.SecretTypeOpaque,
	}

	// update secret or create if not exist
	_, err = k8sClient.CoreV1().Secrets("d8-cloud-instance-manager").Get(context.TODO(), "packages-proxy-token", v1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err := k8sClient.CoreV1().Secrets("d8-cloud-instance-manager").Create(context.TODO(), secret, v1.CreateOptions{})
		return token, err
	}
	_, err = k8sClient.CoreV1().Secrets("d8-cloud-instance-manager").Update(context.TODO(), secret, v1.UpdateOptions{})
	return token, err
}
