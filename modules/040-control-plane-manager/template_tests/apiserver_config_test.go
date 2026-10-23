/*
Copyright 2026 Flant JSC

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

package template_tests

// The kube-apiserver configuration files are checked with the code of kube-apiserver itself: decoded
// strictly and validated, which compiles every CEL expression in them. A field the API server does
// not know, or an expression it cannot compile, stops kube-apiserver on every master, and a string
// comparison of the rendered file would not notice either.

import (
	"context"
	"fmt"
	"os"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/traits"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiserverapi "k8s.io/apiserver/pkg/apis/apiserver"
	apiserverinstall "k8s.io/apiserver/pkg/apis/apiserver/install"
	authorizationload "k8s.io/apiserver/pkg/apis/apiserver/load"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	authenticationcel "k8s.io/apiserver/pkg/authentication/cel"
	authorizationcel "k8s.io/apiserver/pkg/authorization/cel"
)

// loadAuthenticationConfiguration decodes an AuthenticationConfiguration as kube-apiserver does and
// validates it.
func loadAuthenticationConfiguration(data []byte) (*apiserverapi.AuthenticationConfiguration, error) {
	scheme := runtime.NewScheme()
	apiserverinstall.Install(scheme)
	codecs := serializer.NewCodecFactory(scheme, serializer.EnableStrict)
	obj, _, err := codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	config, ok := obj.(*apiserverapi.AuthenticationConfiguration)
	if !ok {
		return nil, fmt.Errorf("decoded %T, not an AuthenticationConfiguration", obj)
	}
	if errs := apiservervalidation.ValidateAuthenticationConfiguration(authenticationcel.NewDefaultCompiler(), config, nil); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return config, nil
}

// extrasFor evaluates extra claim mappings against the claims of a token and returns the extras
// kube-apiserver gives the user. An expression that fails is an error: kube-apiserver rejects the
// whole token then. null and "" leave the key out, as in kube-apiserver.
//
// Claims are given as encoding/json decodes them, so numbers are float64 - doubles in CEL.
func extrasFor(mappings []apiserverapi.ExtraMapping, claims map[string]interface{}) (map[string][]string, error) {
	compiler := authenticationcel.NewDefaultCompiler()
	compiled := make([]authenticationcel.CompilationResult, 0, len(mappings))
	for _, mapping := range mappings {
		result, err := compiler.CompileClaimsExpression(&authenticationcel.ExtraMappingExpression{
			Key:        mapping.Key,
			Expression: mapping.ValueExpression,
		})
		if err != nil {
			return nil, fmt.Errorf("compile %q: %w", mapping.ValueExpression, err)
		}
		compiled = append(compiled, result)
	}

	claimsValue, ok := types.DefaultTypeAdapter.NativeToValue(claims).(traits.Mapper)
	if !ok {
		return nil, fmt.Errorf("claims are not a CEL map")
	}
	evaluated, err := authenticationcel.NewClaimsMapper(compiled).EvalClaimMappings(context.Background(), claimsValue)
	if err != nil {
		return nil, err
	}

	extras := map[string][]string{}
	for _, result := range evaluated {
		key := result.ExpressionAccessor.(*authenticationcel.ExtraMappingExpression).Key
		switch result.EvalResult.Type() {
		case types.NullType:
		case types.StringType:
			if value := result.EvalResult.Value().(string); value != "" {
				extras[key] = []string{value}
			}
		default:
			return nil, fmt.Errorf("%s: expression must return a string or null, returned %s", key, result.EvalResult.Type().TypeName())
		}
	}
	return extras, nil
}

// loadAuthorizationConfiguration decodes an AuthorizationConfiguration as kube-apiserver does and
// validates it with the authorizer types kube-apiserver knows. Validation opens the kubeconfig of
// every webhook, so a copy of the configuration pointing at an empty file is validated; the
// configuration returned keeps the paths it was rendered with.
func loadAuthorizationConfiguration(data []byte) (*apiserverapi.AuthorizationConfiguration, error) {
	config, err := authorizationload.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	kubeconfig, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeconfig.Name())
	if err := kubeconfig.Close(); err != nil {
		return nil, err
	}
	validated := config.DeepCopy()
	for i := range validated.Authorizers {
		if webhook := validated.Authorizers[i].Webhook; webhook != nil && webhook.ConnectionInfo.KubeConfigFile != nil {
			path := kubeconfig.Name()
			webhook.ConnectionInfo.KubeConfigFile = &path
		}
	}
	knownTypes := sets.New("AlwaysAllow", "AlwaysDeny", "ABAC", "Webhook", "RBAC", "Node")
	repeatableTypes := sets.New("Webhook")
	errs := apiservervalidation.ValidateAuthorizationConfiguration(authorizationcel.NewDefaultCompiler(), field.NewPath(""), validated, knownTypes, repeatableTypes)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return config, nil
}

// webhookMatches evaluates the match conditions of a webhook authorizer against a review, as
// kube-apiserver does before it calls the webhook.
func webhookMatches(webhook *apiserverapi.WebhookConfiguration, review *authorizationv1.SubjectAccessReview) (bool, error) {
	matcher, errs := apiservervalidation.ValidateAndCompileMatchConditions(authorizationcel.NewDefaultCompiler(), webhook.MatchConditions)
	if len(errs) > 0 {
		return false, errs.ToAggregate()
	}
	if matcher == nil {
		return true, nil
	}
	return matcher.Eval(context.Background(), review)
}
