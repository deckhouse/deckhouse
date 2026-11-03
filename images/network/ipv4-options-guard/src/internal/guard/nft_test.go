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
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

func TestNftIntact(t *testing.T) {
	forgetRenderings()
	st := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	active := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}
	if !nftIntact(active, nftChain(nil), nftRules(st), st) {
		t.Fatal("the filter as written is not intact")
	}

	prio := nftables.ChainPriority(0)
	chains := map[string]func(*nftables.Chain){
		"no hook":        func(c *nftables.Chain) { c.Hooknum = nil },
		"no priority":    func(c *nftables.Chain) { c.Priority = nil },
		"other hook":     func(c *nftables.Chain) { c.Hooknum = nftables.ChainHookInput },
		"other priority": func(c *nftables.Chain) { c.Priority = &prio },
		"other type":     func(c *nftables.Chain) { c.Type = nftables.ChainTypeNAT },
	}
	for name, change := range chains {
		ch := nftChain(nil)
		change(ch)
		if nftIntact(active, ch, nftRules(st), st) {
			t.Errorf("%s: reported as intact", name)
		}
	}
	if nftIntact(active, nil, nftRules(st), st) {
		t.Error("missing chain reported as intact")
	}
	// A dormant table keeps its rules but its base chains see no packets.
	dormant := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4, Flags: unix.NFT_TABLE_F_DORMANT}
	if nftIntact(dormant, nftChain(nil), nftRules(st), st) || nftIntact(nil, nftChain(nil), nftRules(st), st) {
		t.Error("dormant or missing table reported as intact")
	}

	exprs := map[string]func([]expr.Any){
		"payload register": func(e []expr.Any) { e[0].(*expr.Payload).DestRegister = 2 },
		"payload write":    func(e []expr.Any) { e[0].(*expr.Payload).OperationType = expr.PayloadWrite },
		"bitwise len":      func(e []expr.Any) { e[3].(*expr.Bitwise).Len = 2 },
		"cmp register":     func(e []expr.Any) { e[1].(*expr.Cmp).Register = 2 },
		"cmp data":         func(e []expr.Any) { e[1].(*expr.Cmp).Data = []byte{17} },
		"verdict":          func(e []expr.Any) { e[len(e)-1].(*expr.Verdict).Kind = expr.VerdictAccept },
	}
	for name, change := range exprs {
		rules := nftRules(st)
		change(rules[0].Exprs)
		if nftIntact(active, nftChain(nil), rules, st) {
			t.Errorf("%s: reported as intact", name)
		}
	}

	// Counter values change with traffic and are ignored.
	rules := nftRules(st)
	for _, r := range rules {
		for _, e := range r.Exprs {
			if c, ok := e.(*expr.Counter); ok {
				c.Packets, c.Bytes = 10, 600
			}
		}
	}
	if !nftIntact(active, nftChain(nil), rules, st) {
		t.Error("counter values make the filter look broken")
	}
}

func forgetRenderings() {
	renderings.Lock()
	defer renderings.Unlock()
	renderings.m = map[string]string{}
}

func TestNftIntactReadBack(t *testing.T) {
	forgetRenderings()
	t.Cleanup(forgetRenderings)
	st := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	active := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}
	// The kernel lists the payload with a field the guard does not set.
	listed := nftRules(st)
	listed[0].Exprs[0].(*expr.Payload).CsumFlags = 1
	if nftIntact(active, nftChain(nil), listed, st) {
		t.Fatal("a different listing is intact before it was read back")
	}
	rememberRendering(nftRenderingKey(st.Mode), nftRendering(listed))
	if !nftIntact(active, nftChain(nil), listed, st) {
		t.Fatal("the listing read back after the write is not intact")
	}
	if !nftIntact(active, nftChain(nil), nftRules(st), st) {
		t.Fatal("the expected form is no longer intact")
	}
	// Any other change still is not.
	changed := nftRules(st)
	changed[0].Exprs[0].(*expr.Payload).CsumFlags = 1
	changed[0].Exprs[len(changed[0].Exprs)-1].(*expr.Verdict).Kind = expr.VerdictAccept
	if nftIntact(active, nftChain(nil), changed, st) {
		t.Fatal("a changed verdict is intact")
	}
	if nftIntact(active, nftChain(nil), listed, State{Mode: ModeAudit, MPTCPOrig: "1"}) {
		t.Fatal("the listing of Enforce is intact in Audit")
	}
}

func TestNftRepaired(t *testing.T) {
	forgetRenderings()
	t.Cleanup(forgetRenderings)
	st := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	active := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}
	// The kernel lists a match with a field the guard does not set.
	listed := nftRules(st)
	listed[0].Exprs[0].(*expr.Payload).CsumFlags = 1
	accept := nftRules(st)
	accept[0].Exprs[len(accept[0].Exprs)-1].(*expr.Verdict).Kind = expr.VerdictAccept

	if nftRepaired(nil, nil, nil, st) {
		t.Error("first install reported as a repair")
	}
	if nftRepaired(active, nftChain(nil), nftRules(State{Mode: ModeAudit, MPTCPOrig: "1"}), st) {
		t.Error("mode change reported as a repair")
	}
	if !nftRepaired(active, nftChain(nil), accept, st) {
		t.Error("changed verdict not reported")
	}
	if !nftRepaired(active, nftChain(nil), nftRules(st)[:1], st) {
		t.Error("removed rule not reported")
	}
	// After a restart our own listing is unknown: a different one is rewritten without a report.
	if nftRepaired(active, nftChain(nil), listed, st) {
		t.Error("a different listing reported as a repair after a restart")
	}
	rememberRendering(nftRenderingKey(st.Mode), nftRendering(nftRules(st)))
	if !nftRepaired(active, nftChain(nil), listed, st) {
		t.Error("changed match not reported once our listing is known")
	}
}
