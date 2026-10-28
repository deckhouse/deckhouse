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

package commands

import (
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func DefineTestSSHConnectionCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		sshProviderInitializer, err := providerinitializer.CLISSHProviderInitializer(ctx, opts)
		if err != nil {
			return err
		}
		if sshProviderInitializer == nil {
			return fmt.Errorf("SSH credentials not provided")
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}
		sshCl, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		if err := sshCl.Check().AwaitAvailability(ctx, retry.NewEmptyParams()); err != nil {
			return fmt.Errorf("check connection: %v", err)
		}

		TestCommandDelay()

		return nil
	}
}

func DefineTestSCPCommand(cmd *cobra.Command, opts *options.Options) {
	var SrcPath string
	var DstPath string
	var Data string
	var Direction string

	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)

	flags := cmd.Flags()
	app.StringFlag(flags, app.Flag{Name: "src", Short: "s", Help: "source path"}, &SrcPath)
	app.StringFlag(flags, app.Flag{Name: "dst", Short: "d", Help: "destination path"}, &DstPath)
	app.StringFlag(flags, app.Flag{Name: "data", Help: "data to test uploadbytes method"}, &Data)
	app.StringFlag(flags, app.Flag{Name: "way", Short: "w", Help: "transfer direction: 'up' to upload to remote or 'down' to download from remote"}, &Direction)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		sshProviderInitializer, err := providerinitializer.CLISSHProviderInitializer(ctx, opts)
		if err != nil {
			return err
		}
		if sshProviderInitializer == nil {
			return fmt.Errorf("SSH credentials not provided")
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}
		sshCl, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		l.Debug("scp: start")

		success := false
		if Direction == "up" {
			if Data != "" {
				l.Info(fmt.Sprintf("upload bytes to '%s' on remote", DstPath))
				err = sshCl.File().UploadBytes(ctx, []byte(Data), DstPath)
			} else {
				l.Info(fmt.Sprintf("upload local '%s' to '%s' on remote", SrcPath, DstPath))
				err = sshCl.File().Upload(ctx, SrcPath, DstPath)
			}
			if err != nil {
				return err
			}
			success = true
		} else {
			if DstPath == "stdout" {
				l.Info(fmt.Sprintf("download bytes from remote '%s'", SrcPath))
				data, err := sshCl.File().DownloadBytes(ctx, SrcPath)
				if err != nil {
					return err
				}
				l.Info(fmt.Sprint(string(data)))
				success = true
			} else {
				l.Info(fmt.Sprintf("download bytes from remote '%s' to local '%s'", SrcPath, DstPath))
				err = sshCl.File().Download(ctx, SrcPath, DstPath)
				if err != nil {
					return err
				}
				success = true
			}
		}

		if !success {
			l.Info("unrecognized flags")
		}

		return nil
	}
}

func DefineTestUploadExecCommand(cmd *cobra.Command, opts *options.Options) {
	var ScriptPath string
	var Sudo bool

	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.StringFlag(cmd.Flags(), app.Flag{Name: "script", Help: "source path"}, &ScriptPath)
	app.BoolFlag(cmd.Flags(), app.Flag{Name: "sudo", Short: "s", Help: "source path"}, &Sudo)

	cmd.RunE = func(c *cobra.Command, _ []string) error {
		ctx := c.Context()
		l := logger.FromContext(ctx)

		sshProviderInitializer, err := providerinitializer.CLISSHProviderInitializer(ctx, opts)
		if err != nil {
			return err
		}
		if sshProviderInitializer == nil {
			return fmt.Errorf("SSH credentials not provided")
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}
		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		cmd := sshClient.UploadScript(ScriptPath)
		if Sudo {
			cmd.Sudo()
		}

		var stdout []byte
		stdout, err = cmd.Execute(ctx)
		if err != nil {
			if ee, ok := errors.AsType[*exec.ExitError](err); ok {
				return fmt.Errorf("script '%s' error: %w\nstderr: %s", ScriptPath, err, string(ee.Stderr))
			}
			return fmt.Errorf("script '%s' error: %w", ScriptPath, err)
		}

		l.Info(fmt.Sprintf("stdout: %s", strings.Trim(string(stdout), "\n ")))
		l.Info(fmt.Sprintf("Got %d symbols", len(stdout)))

		return nil
	}
}

func DefineTestBundle(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)

	var bundleDirFlag string
	var scriptNameFlag string

	app.StringFlag(cmd.Flags(), app.Flag{Name: "bundle-dir", Short: "d", Help: "path of a bundle root directory"}, &bundleDirFlag)
	app.StringFlag(cmd.Flags(), app.Flag{Name: "bundle-script", Short: "s", Help: "path of a bundle main script"}, &scriptNameFlag)

	cmd.RunE = func(c *cobra.Command, _ []string) error {
		ctx := c.Context()
		l := logger.FromContext(ctx)

		sshProviderInitializer, err := providerinitializer.CLISSHProviderInitializer(ctx, opts)
		if err != nil {
			return err
		}
		if sshProviderInitializer == nil {
			return fmt.Errorf("SSH credentials not provided")
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}
		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		cmd := sshClient.UploadScript(scriptNameFlag)
		cmd.Sudo()

		parentDir := path.Dir(bundleDirFlag)
		bundleDir := path.Base(bundleDirFlag)

		stdout, err := cmd.ExecuteBundle(ctx, parentDir, bundleDir)
		if err != nil {
			if ee, ok := errors.AsType[*exec.ExitError](err); ok {
				return fmt.Errorf("bundle '%s' error: %w\nstderr: %s\n", bundleDir, err, string(ee.Stderr))
			}
			return fmt.Errorf("bundle '%s' error: %w", bundleDir, err)
		}

		l.Info(fmt.Sprintf("Got %d symbols", len(stdout)))

		return nil
	}
}
