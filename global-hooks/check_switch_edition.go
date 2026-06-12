// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// order 4 needs to after deckhouse_edition
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 4},
}, dependency.WithExternalDependencies(checkSwitchDeckhouseEdition))

type switchEditionCheckerParams struct {
	currentEdition     string
	previousEdition    string
	eeEditions         map[string]struct{}
	allEditionsInOrder []string
}

func (p *switchEditionCheckerParams) currentIsEE() bool {
	_, ok := p.eeEditions[p.currentEdition]
	return ok
}

func (p *switchEditionCheckerParams) previousIsEE() bool {
	_, ok := p.eeEditions[p.previousEdition]
	return ok
}

func (p *switchEditionCheckerParams) previousIs(edition string) bool {
	return p.previousEdition == edition
}

type switchEditionChecker func(context.Context, *go_hook.HookInput, dependency.Container, *switchEditionCheckerParams) error

var switchEditionCheckers = []switchEditionChecker{
	controlPlaneSwitchEditionChecker,
}

func checkSwitchDeckhouseEdition(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	currentEdition := input.Values.Get("global.deckhouseEdition").String()
	if currentEdition == UnknownEdition || currentEdition == "" {
		return fmt.Errorf("Have incorrect edition '%s'. Cannot continue", currentEdition)
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}

	d8Deployment, err := client.AppsV1().Deployments("d8-system").Get(ctx, "deckhouse", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot get deckhouse deployment: %v", err)
	}

	annotations := d8Deployment.GetAnnotations()
	if len(annotations) == 0 {
		input.Logger.Warn("Empty annotations. Probably install cluster. Skip")
		return nil
	}

	previousEdition, ok := annotations["core.deckhouse.io/edition"]
	if !ok || previousEdition == "" {
		input.Logger.Warn("Not found edition annotations. Probably install cluster. Skip")
		return nil
	}

	if previousEdition == UnknownEdition {
		input.Logger.Warn("Previous edition is Unknown. Skip")
		return nil
	}

	input.Logger.Info(
		"Discovered editions",
		slog.String("previous", previousEdition),
		slog.String("current", currentEdition),
	)

	if currentEdition == previousEdition {
		input.Logger.Info("Previous edition and current is same. Skip")
		return nil
	}

	params := newSwitchEditionCheckerParams(previousEdition, currentEdition, input)

	var errors []error

	for _, checker := range switchEditionCheckers {
		err := checker(ctx, input, dc, params)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) == 0 {
		input.Logger.Info(
			"All switch edition check passed. Allow switch edition",
			slog.Int("checks_count", len(switchEditionCheckers)),
			slog.String("from", previousEdition),
			slog.String("to", currentEdition),
		)
		return nil
	}

	resErr := errors[0]
	for _, nextErr := range errors[1:] {
		resErr = fmt.Errorf("\n%w", nextErr)
	}

	return fmt.Errorf(
		"Disallow switch edition from %s to %s: %w",
		previousEdition,
		currentEdition,
		resErr,
	)
}

func newSwitchEditionCheckerParams(previous, current string, input *go_hook.HookInput) *switchEditionCheckerParams {
	return &switchEditionCheckerParams{
		previousEdition:    previous,
		currentEdition:     current,
		eeEditions:         extractEEEditionsFromBuildFlag(input.Logger),
		allEditionsInOrder: extractAllEditionsInOrder(input.Logger),
	}
}

func controlPlaneSwitchEditionChecker(
	ctx context.Context,
	input *go_hook.HookInput,
	dc dependency.Container,
	p *switchEditionCheckerParams,
) error {
	allowSwitch := func(input *go_hook.HookInput, cause string, args ...any) error {
		cause = fmt.Sprintf(cause, args...)
		input.Logger.Info(
			"Allow switch edition by control-plane checker", slog.String("cause", cause),
		)
		return nil
	}

	disallowSwitch := func(p *switchEditionCheckerParams, cause string, args ...any) error {
		cause = fmt.Sprintf(cause, args...)
		return fmt.Errorf(
			"%s. It will break control-plane. Previous edition %s, current is %s",
			cause,
			p.previousEdition,
			p.currentEdition,
		)
	}

	// upgrade to ee-like
	if !p.previousIsEE() {
		additionalInfo := "between non EE-like editions"
		if p.currentIsEE() {
			additionalInfo = "from not EE-like to EE-like"
		}
		return allowSwitch(input, "Previous is not EE-like allow switch %s", additionalInfo)
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot get k8s client for check ds: %w", err)
	}

	cpmDs, err := client.AppsV1().DaemonSets("kube-system").Get(ctx, "d8-control-plane-manager", metav1.GetOptions{})
	if err != nil {
		// managed clusters case
		if k8serrors.IsNotFound(err) {
			return allowSwitch(input, "kube-system/d8-control-plane-manager ds not found")
		}

		return fmt.Errorf("cannot get kube-system/d8-control-plane-manager ds: %w", err)
	}

	annotations := cpmDs.GetAnnotations()
	if len(annotations) == 0 {
		return allowSwitch(input, "kube-system/d8-control-plane-manager ds has not annotations")
	}

	enabledVal, ok := annotations["signature/enabled"]
	if !ok {
		return allowSwitch(input, "kube-system/d8-control-plane-manager ds has not sign annotation")
	}

	enabled := strings.ToLower(enabledVal) == "true"

	// if not signature enabled can switch to not EE-like and in EE-likes editions
	if !enabled {
		return allowSwitch(input, "got signature is disabled from kube-system/d8-control-plane-manager ds")
	}

	// signature enabled

	// prev ee-like. downgrade from ee-like with enabled signature
	// is not allowed
	if !p.currentIsEE() {
		return disallowSwitch(p, "Previous is EE-like, but current is not EE-like and sign enabled")
	}

	// current and prev is ee allow switch from ee to cse
	// and in fe and ee
	if !p.previousIs("CSE") {
		return allowSwitch(input, "Migrate between EE-like but not cse")
	}

	// downgrade from cse to ee

	mode, err := extractSignatureModeFromMC(ctx, client, input.Logger)
	if err != nil {
		return err
	}

	if mode == "" || mode == "Rollback" {
		return disallowSwitch(p, "Get signature mode '%s'. Potential rollback", mode)
	}

	return allowSwitch(input, "Got signature mode '%s' Allow switch from CSE to EE-like", mode)
}

func extractSignatureModeFromMC(ctx context.Context, kubeCl k8s.Client, logger go_hook.Logger) (string, error) {
	moduleConfig, err := kubeCl.Dynamic().Resource(moduleConfigGVR).Get(ctx, "control-plane-manager", metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("ModuleConfig for control-plane-manager does not exists, returns no signature mode")
			return "", nil
		}

		return "", fmt.Errorf("cannot get mc for control-plane-manager: %w", err)
	}

	enabledRes, err := moduleEnabledByModuleConfig(moduleConfig)
	if err != nil {
		return "", fmt.Errorf("cannot extract enabled flag from mc for control-plane manager")
	}

	if enabledRes.enabledFlagExists && !enabledRes.enabled {
		logger.Info("ModuleConfig for control-plane-manager not enabled")
		return "", nil
	}

	mode, _, err := moduleConfigSettingString(moduleConfig, "apiserver", "signature")
	if err != nil {
		return "", fmt.Errorf("cannot get signature mode for control-plane-manager: %w", err)
	}

	return mode, nil
}
