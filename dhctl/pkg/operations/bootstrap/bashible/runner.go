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

package bashible

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry/relay"
)

const (
	endPipelineFileMark = app.NodeDeckhouseDirectoryPath + "/first-control-plane-bashible-ran"

	// bundleStepsStatusDir must match STEPS_STATUS_DIR set up in bashible.sh.tpl.
	bundleStepsStatusDir = "/var/lib/bashible/bundle_steps_status"

	stepsStatusPollInterval = 15 * time.Second
)

var (
	stepsStatusNameRe     = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	stepsStatusChecksumRe = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

var (
	alreadyRunDefaultOpts      = retry.AttemptsWithWaitOpts(300, 1*time.Second)
	prepareDefaultOpts         = retry.AttemptsWithWaitOpts(300, 1*time.Second)
	executeBundleDefaultOpts   = retry.AttemptsWithWaitOpts(100, 1*time.Second)
	readFileForInfoDefaultOpts = retry.AttemptsWithWaitOpts(30, 1*time.Second)
)

type LoopsParams struct {
	AlreadyRun      retry.Params
	Prepare         retry.Params
	ExecuteBundle   retry.Params
	ReadFileForInfo retry.Params
}

type NodeInfo struct {
	NodeName string
	NodeIP   string
}

type Runner struct {
	logger        *slog.Logger
	nodeInterface libcon.Interface
	loopsParams   LoopsParams
}

func NewRunner(nodeInterface libcon.Interface, logger *slog.Logger) *Runner {
	return &Runner{
		nodeInterface: nodeInterface,
		logger:        logger,
	}
}

func (r *Runner) WithLoopParams(p LoopsParams) *Runner {
	r.loopsParams = p
	return r
}

func (r *Runner) Prepare(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "BashibleRunner.Prepare")
	defer span.End()

	if err := r.createDir(ctx, app.NodeDeckhouseDirectoryPath, "0755"); err != nil {
		return err
	}

	if err := r.createDir(ctx, app.DeckhouseNodeTmpPath, "1777"); err != nil {
		return err
	}

	// in end of pipeline steps bashible write "OK" to this file
	// we need creating it before because we do not want handle errors from cat
	return r.touchFile(ctx, endPipelineFileMark)
}

func (r *Runner) AlreadyRun(ctx context.Context) (bool, error) {
	loopParams := retry.SafeCloneOrNewParams(r.loopsParams.AlreadyRun, alreadyRunDefaultOpts...).
		Clone(
			retry.WithName("Checking whether Bashible already ran"),
			retry.WithLogger(dhlog.FromContext(ctx)),
		)

	isReady := false

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		cmd := r.nodeInterface.Command("cat", endPipelineFileMark)
		cmd.Sudo(ctx)
		cmd.WithTimeout(10 * time.Second)
		stdout, stderr, err := cmd.Output(ctx)
		if err != nil {
			return err
		}

		r.logger.DebugContext(ctx, fmt.Sprintf("cat %s stdout: '%s'; stderr: '%s'\n", endPipelineFileMark, stdout, stderr))

		isReady = strings.Contains(string(stdout), "OK")

		return nil
	})

	return isReady, err
}

func (r *Runner) ReadNodeInfo(ctx context.Context) (*NodeInfo, error) {
	res := NodeInfo{}

	infoFiles := map[string]*string{
		"/var/lib/bashible/discovered-node-name": &res.NodeName,
		"/var/lib/bashible/discovered-node-ip":   &res.NodeIP,
	}

	for fileName, resPointer := range infoFiles {
		loopParams := retry.SafeCloneOrNewParams(r.loopsParams.ReadFileForInfo, readFileForInfoDefaultOpts...).
			Clone(
				retry.WithName("Read info file %s", fileName),
				retry.WithLogger(dhlog.FromContext(ctx)),
			)

		err := retry.NewLoopWithParams(loopParams).
			RunContext(ctx, func() error {
				f := r.nodeInterface.File()
				content, err := f.DownloadBytes(ctx, fileName)
				if err != nil {
					return err
				}

				contentStr := strings.TrimSpace(string(content))

				// TODO handle in lib-connection
				// Sudo-wrapped commands prefix their stdout with the SUDO-SUCCESS marker;
				// strip everything up to and including the last occurrence so we keep only
				// the actual file payload. For non-sudo paths the marker is absent and
				// output stays untouched.
				if idx := strings.LastIndex(contentStr, "SUDO-SUCCESS"); idx >= 0 {
					contentStr = contentStr[idx+len("SUDO-SUCCESS"):]
				}

				*resPointer = contentStr
				return nil
			})

		if err != nil {
			return nil, err
		}
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Got node info %+v", res))

	return &res, nil
}

type ExecuteBundleParams struct {
	BundleDir     string
	CommanderMode bool
	GlobalOpts    *options.GlobalOptions

	// OnStepsStatus, if set, is called with the currently known set of
	// completed bashible bundle steps (name -> content checksum), both
	// periodically while the bundle is executing and right after each
	// execution attempt. It lets the caller persist progress so a later
	// dhctl run can resume instead of re-running already-completed steps.
	OnStepsStatus func(ctx context.Context, statuses map[string]string)
}

// FetchStepsStatus reads the bootstrap-only per-step completion markers
// (name -> content checksum) that bb-run-step writes into bundleStepsStatusDir
// on the node. Missing/empty directory is not an error, it just yields an
// empty map.
func (r *Runner) FetchStepsStatus(ctx context.Context) (map[string]string, error) {
	cmd := r.nodeInterface.Command("sh", "-c", fmt.Sprintf(
		`for f in %s/*; do [ -f "$f" ] && printf '%%s %%s\n' "$(basename "$f")" "$(cat "$f")"; done`,
		bundleStepsStatusDir,
	))
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)

	stdout, _, err := cmd.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch bashible bundle steps status: %w", err)
	}

	return parseStepsStatus(string(stdout)), nil
}

func parseStepsStatus(output string) map[string]string {
	statuses := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}

		name, checksum := fields[0], fields[1]
		if !stepsStatusNameRe.MatchString(name) || !stepsStatusChecksumRe.MatchString(checksum) {
			continue
		}

		statuses[name] = checksum
	}

	return statuses
}

// PushStepsStatus seeds bundleStepsStatusDir on the node with previously
// remembered (name -> content checksum) markers before bashible.sh runs, so a
// resumed bootstrap can skip steps that already succeeded with identical
// content, even if the node was recreated since the markers were recorded.
func (r *Runner) PushStepsStatus(ctx context.Context, statuses map[string]string) error {
	if len(statuses) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "mkdir -p %s", bundleStepsStatusDir)

	for name, checksum := range statuses {
		if !stepsStatusNameRe.MatchString(name) || !stepsStatusChecksumRe.MatchString(checksum) {
			return fmt.Errorf("invalid bashible step status entry: name=%q checksum=%q", name, checksum)
		}

		fmt.Fprintf(&b, " && printf '%%s' '%s' > '%s/%s'", checksum, bundleStepsStatusDir, name)
	}

	cmd := r.nodeInterface.Command("sh", "-c", b.String())
	cmd.Sudo(ctx)
	cmd.WithTimeout(30 * time.Second)

	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf("push bashible bundle steps status: %w", err)
	}

	return nil
}

func (r *Runner) ExecuteBundle(ctx context.Context, params ExecuteBundleParams) error {
	ctx, span := telemetry.StartSpan(ctx, "BashibleRunner.ExecuteBundle")
	defer span.End()

	loopParams := retry.SafeCloneOrNewParams(r.loopsParams.ExecuteBundle, executeBundleDefaultOpts...).
		Clone(
			retry.WithName("Execute bundle"),
			retry.WithLogger(dhlog.FromContext(ctx)),
		)

	var relaySpanUpdater = func(trace.Span) {}

	if telemetry.IsEnabled() {
		stopRelay, updateRelaySpan, err := relay.InitRelay(ctx, relay.RelayParams{
			TracerName: "bashible",
			Span:       span,
			Node:       r.nodeInterface,
			Logger:     r.logger,
			GlobalOpts: params.GlobalOpts,
		})
		if err != nil {
			return fmt.Errorf("init OTel relay: %w", err)
		}
		defer stopRelay()
		relaySpanUpdater = updateRelaySpan

		telemetryEnvs := fmt.Sprintf(
			"DHCTL_TELEMETRY_ENABLED=%t\nOTEL_RELAY_ADDRESS=%s\n",
			telemetry.IsEnabled(),
			fmt.Sprintf("http://%s:%s", relay.RelayAddress, relay.RelayPort),
		)
		writeTelemetryCmd := r.nodeInterface.Command(fmt.Sprintf("echo -e %q > /var/lib/bashible/telemetry.env", telemetryEnvs))
		writeTelemetryCmd.Sudo(ctx)

		if err := writeTelemetryCmd.Run(ctx); err != nil {
			r.logger.ErrorContext(ctx, fmt.Sprintf("failed to write telemetry.env: %v", err))
		}
	}

	return retry.NewLoopWithParams(loopParams).
		RunContext(ctx, func() error {
			// we do not need to restart tunnel because we have HealthMonitor
			logger := r.logger

			logger.DebugContext(ctx, "Stopping Bashible if needed")

			if err := r.cleanupPreviousBashibleIfNeed(ctx); err != nil {
				return err
			}

			logger.DebugContext(ctx, "Starting Bashible bundle execution routine")

			return r.attemptExecuteBundle(ctx, params, relaySpanUpdater)
		})
}

func (r *Runner) attemptExecuteBundle(
	ctx context.Context,
	params ExecuteBundleParams,
	spanUpdater func(trace.Span),
) error {
	ctx, span := telemetry.StartSpan(ctx, "BashibleRunner.attemptExecuteBundle")
	defer span.End()

	// we need this, due to not create relay in every attempt, but we need to correct hook data from bashible
	spanUpdater(span)

	stopStepsStatusPolling := r.startStepsStatusPolling(ctx, params.OnStepsStatus)
	defer stopStepsStatusPolling()

	bundleCmd := r.nodeInterface.UploadScript("bashible.sh", "--local")
	bundleCmd.WithCleanupAfterExec(false)
	bundleCmd.Sudo()
	parentDir := params.BundleDir + "/var/lib"
	bundleDir := "bashible"

	_, err := bundleCmd.ExecuteBundle(ctx, parentDir, bundleDir)

	r.reportStepsStatus(ctx, params.OnStepsStatus)

	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return fmt.Errorf("bundle '%s' error: %w\nstderr: %s", bundleDir, err, string(ee.Stderr))
		}

		return fmt.Errorf("bundle '%s' error: %w", bundleDir, err)
	}
	return nil
}

// startStepsStatusPolling periodically reports the node's current steps
// status while a (potentially very long, since a single step retries
// indefinitely without MAX_RETRIES) bundle execution attempt is in flight, so
// progress isn't lost if dhctl is interrupted mid-attempt. The returned func
// stops the polling and must be called once the attempt finishes.
func (r *Runner) startStepsStatusPolling(ctx context.Context, onStatus func(context.Context, map[string]string)) func() {
	if onStatus == nil {
		return func() {}
	}

	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(stepsStatusPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.reportStepsStatus(ctx, onStatus)
			}
		}
	}()

	return func() {
		close(done)
	}
}

func (r *Runner) reportStepsStatus(ctx context.Context, onStatus func(context.Context, map[string]string)) {
	if onStatus == nil {
		return
	}

	statuses, err := r.FetchStepsStatus(ctx)
	if err != nil {
		r.logger.DebugContext(ctx, fmt.Sprintf("failed to fetch bashible bundle steps status: %v", err))
		return
	}

	if len(statuses) == 0 {
		return
	}

	onStatus(ctx, statuses)
}

func (r *Runner) cleanupPreviousBashibleIfNeed(ctx context.Context) error {
	return dhlog.RunProcess(ctx, r.logger, "Clean up previous Bashible run if needed", func(context.Context) error {
		r.logger.DebugContext(ctx, "Getting Bashible PIDs")
		pids, err := r.getBashiblePIDs(ctx)
		if err != nil {
			return err
		}

		r.logger.DebugContext(ctx, fmt.Sprintf("Got Bashible PIDs: %v\n", pids))
		if len(pids) == 0 {
			r.logger.InfoContext(ctx, "Bashible instance not found. Starting it!")
			return nil
		}

		if err := r.killBashible(ctx, pids); err != nil {
			return err
		}

		return r.unlockBashible(ctx)
	})
}

func (r *Runner) getBashiblePIDs(ctx context.Context) ([]string, error) {
	logger := r.logger

	var psStrings []string
	h := func(l string) {
		psStrings = append(psStrings, l)
	}

	bashCmd := `ps a --no-headers -o args:64 -o "|%p"`
	cmd := r.nodeInterface.Command("bash", "-c", bashCmd)
	cmd.WithStdoutHandler(h)
	if err := r.runCmd(ctx, cmd, bashCmd); err != nil {
		return nil, err
	}

	var res []string
	for _, l := range psStrings {
		logger.DebugContext(ctx, fmt.Sprintf("ps string: '%s'\n", l))

		parts := strings.SplitN(l, "|", 2)
		if len(parts) < 2 {
			logger.DebugContext(ctx, "Skipping ps line without PID")
			continue
		}

		if !strings.Contains(parts[0], "bashible.sh") {
			continue
		}

		pid := strings.TrimSpace(parts[1])
		logger.DebugContext(ctx, fmt.Sprintf("Found bashible PID: %s\n", pid))

		res = append(res, pid)
	}

	return res, nil
}

func (r *Runner) killBashible(ctx context.Context, pids []string) error {
	cmd := r.nodeInterface.Command("kill", pids...)
	return r.runCmd(ctx, cmd, "kill"+strings.Join(pids, " "))
}

func (r *Runner) runCmd(ctx context.Context, cmd libcon.Command, desc string) error {
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(ctx); err != nil {
		// ssh exits with the exit status of the remote command or with 255 if an error occurred.
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			r.logger.DebugContext(ctx, fmt.Sprintf("'%s' got exit code: %d and stderr %s", desc, ee.ExitCode(), string(ee.Stderr)))
			if ee.ExitCode() == 255 {
				return err
			}

			return nil
		}
	}

	return nil
}

func (r *Runner) unlockBashible(ctx context.Context) error {
	cmd := r.nodeInterface.Command("rm", "-f", "/var/lock/bashible")
	return r.runCmd(ctx, cmd, "remove lock file")
}

func (r *Runner) createDir(ctx context.Context, dir, access string) error {
	loopParams := r.prepareLoopParams(ctx, dir)

	bashCmd := withUmask("mkdir -p -m %s %s", access, dir)

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		return r.runWithSH(ctx, bashCmd)
	})
	if err != nil {
		return fmt.Errorf("Cannot create %s directory: %w", dir, err)
	}

	return nil
}

func (r *Runner) touchFile(ctx context.Context, file string) error {
	loopParams := r.prepareLoopParams(ctx, file)

	bashCmd := withUmask("touch %s", file)

	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		return r.runWithSH(ctx, bashCmd)
	})
	if err != nil {
		return fmt.Errorf("Cannot touch %s file: %w", file, err)
	}

	return nil
}

func (r *Runner) runWithSH(ctx context.Context, bashCmd string) error {
	cmd := r.nodeInterface.Command("sh", "-c", bashCmd)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf("ssh: %s: %w", bashCmd, err)
	}

	return nil
}

func (r *Runner) prepareLoopParams(ctx context.Context, target string) retry.Params {
	return retry.SafeCloneOrNewParams(r.loopsParams.Prepare, prepareDefaultOpts...).
		Clone(
			retry.WithName("Prepare %s", target),
			retry.WithLogger(dhlog.FromContext(ctx)),
		)
}

func withUmask(f string, args ...any) string {
	return fmt.Sprintf("umask 0022 ; "+f, args...)
}
