/*
Copyright 2023 Flant JSC

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
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/loki",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 10,
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name: "tokens",
			// At 01:01.
			Crontab: "1 1 * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"loki-access-tokens"},
			},
			FilterFunc: lokiAccessTokensFilter,
		},
	},
}, dependency.WithExternalDependencies(handleBearerTokens))

type bearerTokens struct {
	grafanaToken    *jwt.Token
	logShipperToken *jwt.Token
}

func lokiAccessTokensFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	grafanaToken, _, err := jwt.NewParser().ParseUnverified(string(secret.Data["grafanaToken"]), jwt.MapClaims{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse grafana bearer token")
	}

	logShipperToken, _, err := jwt.NewParser().ParseUnverified(string(secret.Data["logShipperToken"]), jwt.MapClaims{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse log-shipper bearer token")
	}

	return &bearerTokens{
		grafanaToken:    grafanaToken,
		logShipperToken: logShipperToken,
	}, nil
}

func handleBearerTokens(input *go_hook.HookInput, dc dependency.Container) error {
	var (
		tokens                        *bearerTokens
		grafanaTokenExpirationTime    *jwt.NumericDate
		logShipperTokenExpirationTime *jwt.NumericDate
	)

	secretSnapshots := input.Snapshots["secret"]
	if len(secretSnapshots) > 0 {
		tokens = secretSnapshots[0].(*bearerTokens)

		var err error
		grafanaTokenExpirationTime, err = tokens.grafanaToken.Claims.GetExpirationTime()
		if err != nil {
			return errors.Wrapf(err, "failed to get grafana token expiration time")
		}

		logShipperTokenExpirationTime, err = tokens.logShipperToken.Claims.GetExpirationTime()
		if err != nil {
			return errors.Wrapf(err, "failed to get log-shipper token expiration time")
		}
	}

	if tokens == nil ||
		grafanaTokenExpirationTime.Time.Before(time.Now().Add(time.Hour*24*2)) ||
		logShipperTokenExpirationTime.Time.Before(time.Now().Add(time.Hour*24*2)) {
		k8sClient, err := dc.GetK8sClient()
		if err != nil {
			return errors.Wrap(err, "failed to get k8s client")
		}

		client := k8sClient.CoreV1().ServiceAccounts("d8-monitoring")

		clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()

		grafanaToken, err := generateBearerToken(context.TODO(), client, "grafana", clusterDomain)
		if err != nil {
			return errors.Wrapf(err, "failed to generate a bearer token for grafana service account")
		}

		client = k8sClient.CoreV1().ServiceAccounts("d8-log-shipper")

		logShipperToken, err := generateBearerToken(context.TODO(), client, "log-shipper", clusterDomain)
		if err != nil {
			return errors.Wrapf(err, "failed to generate a bearer token for log-shipper service account")
		}

		tokens = &bearerTokens{
			grafanaToken:    grafanaToken,
			logShipperToken: logShipperToken,
		}
	}

	input.Values.Set("loki.internal.grafanaToken", tokens.grafanaToken.Raw)
	input.Values.Set("loki.internal.logShipperToken", tokens.logShipperToken.Raw)

	return nil
}

func generateBearerToken(ctx context.Context, client corev1client.ServiceAccountInterface, serviceAccountName string, clusterDomain string) (*jwt.Token, error) {
	// 1 year
	expirationSeconds := int64(60 * 60 * 24 * 365)

	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{fmt.Sprintf("https://kubernetes.default.svc.%s", clusterDomain)},
			ExpirationSeconds: &expirationSeconds,
		},
	}

	tokenRequest, err := client.CreateToken(ctx, serviceAccountName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create bearer token")
	}

	if tokenRequest.Status.Token == "" {
		return nil, errors.Wrapf(err, "bearer token is empty")
	}

	token, _, err := jwt.NewParser().ParseUnverified(tokenRequest.Status.Token, jwt.MapClaims{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse bearer token")
	}

	return token, nil
}
