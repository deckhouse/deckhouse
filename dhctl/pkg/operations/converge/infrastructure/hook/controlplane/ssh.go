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
// limitations under the License.package controlplane

package controlplane

import (
	"context"
	"fmt"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
)

type SSHChecker struct {
	sshProvider      libcon.SSHProvider
	nodesExternalIPs map[string]string
}

func NewSSHChecker(
	sshProvider libcon.SSHProvider,
	nodesExternalIPs map[string]string,
) *SSHChecker {
	return &SSHChecker{
		sshProvider:      sshProvider,
		nodesExternalIPs: nodesExternalIPs,
	}
}

func (c *SSHChecker) IsReady(ctx context.Context, nodeName string) (bool, error) {
	if c.sshProvider == nil {
		return false, fmt.Errorf("SSH checker: SSH provider is not configured")
	}

	address, ok := c.nodesExternalIPs[nodeName]
	if !ok || address == "" {
		return false, fmt.Errorf(
			"SSH checker: no SSH address found for node %s",
			nodeName,
		)
	}

	client, err := c.clientForNode(ctx, nodeName, address)
	if err != nil {
		return false, fmt.Errorf("SSH checker: %w", err)
	}

	cmd := client.Command("true")
	if err := cmd.Run(ctx); err != nil {
		return false, fmt.Errorf(
			"SSH checker: command failed on node %s: %w; stderr: %s",
			nodeName,
			err,
			string(cmd.StderrBytes()),
		)
	}

	return true, nil
}

func (c *SSHChecker) Name() string {
	return "SSH access is available"
}

func (c *SSHChecker) clientForNode(
	ctx context.Context,
	nodeName string,
	address string,
) (libcon.SSHClient, error) {
	sourceClient, err := c.sshProvider.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("get source SSH client: %w", err)
	}
	if sourceClient == nil {
		return nil, fmt.Errorf("source SSH client is nil")
	}

	sourceSession := sourceClient.Session()
	if sourceSession == nil {
		return nil, fmt.Errorf("source SSH session is nil")
	}

	checkSession := sourceSession.Copy()
	checkSession.SetAvailableHosts([]session.Host{
		{
			Host: address,
			Name: nodeName,
		},
	})

	standaloneProvider, ok := c.sshProvider.(libcon.StandaloneClientProvider)
	if !ok {
		return nil, fmt.Errorf(
			"SSH provider does not support keyed standalone clients",
		)
	}

	client, err := standaloneProvider.StandaloneClientFor(
		ctx,
		SSHCheckerClientKey(nodeName),
		checkSession,
		sourceClient.PrivateKeys(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get standalone client for node %s: %w",
			nodeName,
			err,
		)
	}

	return client, nil
}

func SSHCheckerClientKey(nodeName string) string {
	return "controlplane-readiness/" + nodeName
}
