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
	"encoding/json"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "order_bootstrap_token",
			Crontab: "23 * * * *",
		},
	},
	Queue: "/modules/node-manager/order_bootstrap_token",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "ngs",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "NodeGroup",
			ExecuteHookOnEvents: pointer.Bool(false),
			FilterFunc:          bootstrapTokenFilterNodeGroup,
		},
		{
			Name:                "bootstrap_tokens",
			ApiVersion:          "v1",
			Kind:                "Secret",
			ExecuteHookOnEvents: pointer.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node-manager.deckhouse.io/node-group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: bootstrapTokenFilterSecret,
		},
	},
}, handleOrderBootstrapToken)

func bootstrapTokenFilterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	ng := sec.Labels["node-manager.deckhouse.io/node-group"]

	var validFor time.Duration
	if sec.Type == corev1.SecretTypeBootstrapToken {
		expireRaw, ok := sec.Data["expiration"]
		if ok {
			expire, err := time.Parse(time.RFC3339, string(expireRaw))
			if err != nil {
				return nil, err
			}

			validFor = time.Until(expire)
		}
	}

	var bootstrapToken string
	tokenIDRaw, hasID := sec.Data["token-id"]
	tokenSecretRaw, hasSecret := sec.Data["token-secret"]
	if hasID && hasSecret {
		bootstrapToken = fmt.Sprintf("%s.%s", tokenIDRaw, tokenSecretRaw)
	}

	return bootstrapTokenSecret{
		Name:           sec.Name,
		NodeGroup:      ng,
		ValidFor:       validFor,
		BootstrapToken: bootstrapToken,
		CreationTS:     sec.CreationTimestamp.Time,
	}, nil
}

func bootstrapTokenFilterNodeGroup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	// TODO  maybe need to revert?
	return bootstrapTokenNG{
		Name:      ng.Name,
		NeedToken: true,
	}, err
}

type bootstrapTokenNG struct {
	Name      string
	NeedToken bool
}

type bootstrapTokenSecret struct {
	Name           string
	NodeGroup      string
	ValidFor       time.Duration
	BootstrapToken string
	CreationTS     time.Time
}

func handleOrderBootstrapToken(input *go_hook.HookInput) error {
	tokensByNg := make(map[string]bootstrapTokenSecret)
	expiredTokens := make([]bootstrapTokenSecret, 0)

	snap := input.Snapshots["bootstrap_tokens"]
	for _, sn := range snap {
		token := sn.(bootstrapTokenSecret)
		if token.ValidFor < 0 {
			expiredTokens = append(expiredTokens, token)
			continue
		}

		if latestToken, ok := tokensByNg[token.NodeGroup]; ok {
			// take the latest one
			if token.CreationTS.After(latestToken.CreationTS) {
				tokensByNg[token.NodeGroup] = token
			}
		} else {
			tokensByNg[token.NodeGroup] = token
		}
	}

	// Remove all expired tokens
	for _, token := range expiredTokens {
		input.PatchCollector.Delete("v1", "Secret", "kube-system", token.Name, object_patch.InBackground())
	}

	// we don't want to keep tokens for deleted NodeGroups
	input.Values.Set("nodeManager.internal.bootstrapTokens", json.RawMessage("{}"))

	snap = input.Snapshots["ngs"]
	for _, sn := range snap {
		ng := sn.(bootstrapTokenNG)
		if !ng.NeedToken {
			continue
		}

		latestToken := tokensByNg[ng.Name]

		if latestToken.ValidFor > 3*time.Hour {
			// token is valid for more than 3 hours — we can use it
			input.Values.Set("nodeManager.internal.bootstrapTokens."+ng.Name, latestToken.BootstrapToken)
		} else {
			// token is not valid for more than 3 hours or doesn't exist — we must generate the new one
			tokenID := pwgen.AlphaNumLowerCase(6)
			tokenSecret := pwgen.AlphaNumLowerCase(16)
			tokenExpiration := time.Now().Add(4 * time.Hour)

			newSecret := bootstrapTokenGenerateSecret(tokenID, tokenSecret, ng.Name, tokenExpiration)
			input.PatchCollector.Create(newSecret)

			input.Values.Set("nodeManager.internal.bootstrapTokens."+ng.Name, fmt.Sprintf("%s.%s", tokenID, tokenSecret))
		}
	}

	return nil
}

func bootstrapTokenGenerateSecret(tokenID, tokenSecret, ngName string, expireAt time.Time) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("bootstrap-token-%s", tokenID),
			Namespace: "kube-system",
			Labels: map[string]string{
				"heritage":                             "deckhouse",
				"module":                               "node-manager",
				"node-manager.deckhouse.io/node-group": ngName,
			},
		},
		Data: map[string][]byte{
			"expiration":                     []byte(expireAt.Format(time.RFC3339)),
			"token-id":                       []byte(tokenID),
			"token-secret":                   []byte(tokenSecret),
			"auth-extra-groups":              []byte("system:bootstrappers:d8-node-manager"),
			"usage-bootstrap-authentication": []byte("true"),
			"usage-bootstrap-signing":        []byte("true"),
		},
		Type: corev1.SecretTypeBootstrapToken,
	}
}
