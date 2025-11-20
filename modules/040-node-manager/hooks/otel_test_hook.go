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
	"net/http"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "otel-test-hook",
			Crontab: "* * * * *",
		},
	},
}, dependency.WithExternalDependencies(otelTestHookLogic))

func otelTestHookLogic(input *go_hook.HookInput, dc dependency.Container) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := dc.GetHTTPClient()

	testURL := "http://deckhouse.d8-system.svc.cluster.local:8080/readyz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		input.Logger.Error("Failed to create HTTP request", slog.String("error", err.Error()))
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	input.Logger.Info("Executing OpenTelemetry test HTTP request",
		slog.String("url", testURL),
		slog.String("method", http.MethodGet))

	resp, err := client.Do(req)
	if err != nil {
		input.Logger.Error("HTTP request failed",
			slog.String("error", err.Error()),
			slog.String("url", testURL))

		return nil
	}
	defer resp.Body.Close()

	input.Logger.Info("OpenTelemetry test HTTP request completed",
		slog.String("status", resp.Status),
		slog.Int("status_code", resp.StatusCode),
		slog.String("url", testURL))

	return nil
}
