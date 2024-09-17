// Copyright 2024 Flant JSC
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

package ssh

type RunScriptReverseTunnelChecker struct {
	client     *Client
	scriptPath string
}

func NewRunScriptReverseTunnelChecker(c *Client, scriptPath string) *RunScriptReverseTunnelChecker {
	return &RunScriptReverseTunnelChecker{
		client:     c,
		scriptPath: scriptPath,
	}
}

func (s *RunScriptReverseTunnelChecker) CheckTunnel() (string, error) {
	script := s.client.UploadScript(s.scriptPath)
	script.Sudo()
	out, err := script.Execute()
	return string(out), err
}

type RunScriptReverseTunnelKiller struct {
	client     *Client
	scriptPath string
}

func NewRunScriptReverseTunnelKiller(c *Client, scriptPath string) *RunScriptReverseTunnelKiller {
	return &RunScriptReverseTunnelKiller{
		client:     c,
		scriptPath: scriptPath,
	}
}

func (s *RunScriptReverseTunnelKiller) KillTunnel() (string, error) {
	script := s.client.UploadScript(s.scriptPath)
	script.Sudo()
	out, err := script.Execute()
	return string(out), err
}
