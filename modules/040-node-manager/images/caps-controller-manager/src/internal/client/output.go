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

package client

import "strings"

const (
	// maxLoggedOutputLines and maxLoggedOutputBytes bound what a failed remote command
	// contributes to the controller log. A bootstrap converge prints hundreds of lines and
	// the same output would otherwise be duplicated into the error, the event and the log.
	maxLoggedOutputLines = 50
	maxLoggedOutputBytes = 16 * 1024

	// maxEventMessageBytes keeps the event message small: events are ordinary API objects
	// and a long message makes them unreadable in kubectl describe.
	maxEventMessageBytes = 256
)

// remoteCommandError carries a tail of the remote command output alongside the error.
//
// ssh returns only stderr on failure, while bashible reports its progress and failures on
// stdout. Without this type the controller log says "failed to exec ssh command" and nothing
// else, and the reason has to be recovered from the node after the fact.
type remoteCommandError struct {
	err    error
	output string
}

func (e *remoteCommandError) Error() string { return e.err.Error() }

func (e *remoteCommandError) Unwrap() error { return e.err }

// EventMessage returns a short message for a Kubernetes event, falling back to the error
// itself when the command produced no output (for example, the connection dropped).
func (e *remoteCommandError) EventMessage() string {
	if message := truncateForEvent(e.output); message != "" {
		return message
	}

	return truncateForEvent(e.err.Error())
}

func combinedOutput(stdout, stderr string) string {
	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

// tailOutput keeps the end of the output: that is where a command that failed reports why.
func tailOutput(output string) string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return ""
	}

	lines := strings.Split(output, "\n")
	if len(lines) > maxLoggedOutputLines {
		lines = lines[len(lines)-maxLoggedOutputLines:]
	}

	tail := strings.Join(lines, "\n")
	if len(tail) > maxLoggedOutputBytes {
		tail = strings.ToValidUTF8(tail[len(tail)-maxLoggedOutputBytes:], "")
	}

	return tail
}

// truncateForEvent collapses the output into a single short line suitable for an event message.
func truncateForEvent(output string) string {
	message := strings.Join(strings.Fields(output), " ")
	if len(message) <= maxEventMessageBytes {
		return message
	}

	return strings.ToValidUTF8(message[:maxEventMessageBytes], "") + "..."
}
