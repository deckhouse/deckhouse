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

package preflightnew

import (
	"fmt"
	"strings"
)

// DocsURL is the anchor every preflight failure points the reader at. A check that has a more
// specific page puts it in Failure.Docs; everything else falls back to this one.
const DocsURL = "https://deckhouse.io/products/kubernetes-platform/documentation/v1/installing/#preflight-checks"

// Failure is what a check returns when it can name what it looked at. The runner renders it as a
// block under the check name, and adds the two lines only it knows — the skip flag and the docs
// anchor. A check that returns a bare error still gets those two lines; it just prints its error
// under `reason:` instead of the fields below.
//
// The fields answer, in order, the four questions a failed preflight leaves the reader with:
// what did you look at, what did you see, what did you want, and what do I type now.
type Failure struct {
	// Checked is the thing inspected, in the reader's own vocabulary: a config field path
	// ("ClusterConfiguration.podSubnetCIDR"), a host ("10.0.0.5 as user \"ubuntu\""), or the
	// command that was run (`sudo -n true` on 10.0.0.5).
	Checked string
	// Observed is what came back. One line, no stack of wrapped errors: the raw cause belongs
	// in Err, which prints under `details:`.
	Observed string
	// Expected is the state that would have passed.
	Expected string
	// Fix is what the reader types or edits. A field path they can search for, or a command
	// they can paste. May span several lines when there is more than one way out.
	Fix string
	// Docs overrides DocsURL for checks with a page of their own.
	Docs string
	// Err is the underlying cause — an *url.Error, an *x509 error, a non-zero exit status. It is
	// printed verbatim under `details:` so a support ticket carries it, and it keeps errors.Is
	// and errors.As working through the Failure.
	Err error
	// StopsPhase says this particular failure leaves the rest of the phase unaskable, whichever
	// check happened to report it. Check.StopsPhaseOnFailure says the same thing about a check;
	// this says it about a finding, and the two exist because the thing being guarded is not
	// always a check.
	//
	// The SSH connection is the case. Every node check is made over it, and the brake used to
	// belong to ssh-credential, which carries StopsPhaseOnFailure. Skip that check by name and
	// the brake goes with it, though the connection is just as absent: every check after it then
	// pays lib-connection's own two-minute retry loop to discover the same thing, twenty times
	// over. The connection is what the phase stands on, so the connection is what stops it.
	StopsPhase bool
}

func (f *Failure) Error() string {
	switch {
	case f.Observed != "":
		return f.Observed
	case f.Err != nil:
		return f.Err.Error()
	default:
		return "check failed"
	}
}

func (f *Failure) Unwrap() error { return f.Err }

// docs returns the anchor this failure points at.
func (f *Failure) docs() string {
	if f.Docs != "" {
		return f.Docs
	}
	return DocsURL
}

// writeFields renders everything the check itself knows. The runner appends `skip:` and `docs:`.
func (f *Failure) writeFields(b *strings.Builder) {
	writeField(b, "checked", f.Checked)
	writeField(b, "observed", f.Observed)
	writeField(b, "expected", f.Expected)
	writeField(b, "fix", f.Fix)
	if f.Err != nil {
		writeField(b, "details", f.Err.Error())
	}
}

func writeField(b *strings.Builder, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	// Continuation lines of a multi-line value (a `fix` offering two ways out) are indented to
	// the width of the label, so the block reads as one field rather than as several.
	indent := strings.Repeat(" ", len(name)+2)
	fmt.Fprintf(b, "%s: %s\n", name, strings.ReplaceAll(value, "\n", "\n"+indent))
}
