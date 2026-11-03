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

package guard

import (
	"os"
	"testing"

	"github.com/google/nftables/expr"
)

func TestMarkerRoundTrip(t *testing.T) {
	for _, st := range []State{
		{Mode: ModeEnforce, MPTCPOrig: "1"},
		{Mode: ModeAudit, MPTCPOrig: mptcpUnmanaged},
		{Mode: ModeEnforce, MPTCPOrig: mptcpAbsent},
	} {
		rule, got, ok := parseMarker(marker("syn", st))
		if !ok || rule != "syn" || got != st {
			t.Fatalf("round trip %+v: got %q %+v %v", st, rule, got, ok)
		}
	}
	for _, bad := range []string{"", "foo", "d8-ipv4-options-guard;rule=syn", "other;rule=syn;mode=Enforce;mptcp=1"} {
		if _, _, ok := parseMarker(bad); ok {
			t.Fatalf("parsed invalid marker %q", bad)
		}
	}
}

func TestParseMode(t *testing.T) {
	for _, s := range []string{"Enforce", "Audit", "Disabled"} {
		if _, err := ParseMode(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ParseMode("enforce"); err == nil {
		t.Fatal("expected error for lowercase mode")
	}
}

func TestNftRulesVerdict(t *testing.T) {
	cases := map[Mode]bool{ModeEnforce: true, ModeAudit: false}
	for mode, wantDrop := range cases {
		rules := nftRules(State{Mode: mode, MPTCPOrig: "1"})
		if len(rules) != len(ruleNames) {
			t.Fatalf("%s: %d rules", mode, len(rules))
		}
		for i, r := range rules {
			if name, _, ok := parseMarker(string(r.UserData)); !ok || name != ruleNames[i] {
				t.Fatalf("%s: rule %d marker %q", mode, i, r.UserData)
			}
			drop, counter := false, false
			for _, e := range r.Exprs {
				switch v := e.(type) {
				case *expr.Verdict:
					drop = v.Kind == expr.VerdictDrop
				case *expr.Counter:
					counter = true
				}
			}
			if drop != wantDrop || !counter {
				t.Fatalf("%s: rule %d drop=%v counter=%v", mode, i, drop, counter)
			}
		}
	}
}

func TestLegacyStateChain(t *testing.T) {
	if got := stateChain(State{Mode: ModeEnforce, MPTCPOrig: mptcpUnmanaged}); got != "D8-IPV4OPT:E:unmanaged" || len(got) > 28 {
		t.Fatalf("unexpected chain name %q", got)
	}
}

func TestLoadSettings(t *testing.T) {
	dir := t.TempDir()
	if st, err := LoadSettings(dir + "/absent.json"); err != nil || st != DefaultSettings() || st.Mode != ModeAudit {
		t.Fatalf("missing file: %+v %v", st, err)
	}
	p := dir + "/s.json"
	if err := os.WriteFile(p, []byte(`{"mode":"Enforce","disableMPTCP":false,"allowKernelModuleLoad":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadSettings(p)
	if err != nil || st != (Settings{Mode: ModeEnforce, DisableMPTCP: false, AllowKernelModuleLoad: true}) {
		t.Fatalf("valid file: %+v %v", st, err)
	}
	if err := os.WriteFile(p, []byte(`{"mode":"enforce"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(p); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestManageMPTCPOnlyInEnforce(t *testing.T) {
	for mode, want := range map[Mode]bool{ModeEnforce: true, ModeAudit: false, ModeDisabled: false} {
		if got := manageMPTCP(Settings{Mode: mode, DisableMPTCP: true}); got != want {
			t.Fatalf("%s: got %v", mode, got)
		}
	}
	if manageMPTCP(Settings{Mode: ModeEnforce, DisableMPTCP: false}) {
		t.Fatal("disableMPTCP=false must not manage MPTCP")
	}
}
