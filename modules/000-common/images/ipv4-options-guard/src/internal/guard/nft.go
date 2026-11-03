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
	"fmt"
	"reflect"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

const (
	tableName = "d8_ipv4_options_guard"
	chainName = "prerouting"
	// Before raw (-300) and after conntrack defrag (-400): sees reassembled packets where conntrack is loaded.
	chainPriority = -310
)

// nftBackend programs native nftables expressions via netlink, so no nft binary and no extra kernel modules are needed.
type nftBackend struct{}

func (nftBackend) Name() string { return "nftables" }

// Apply never reports Bypassed: an accept in another base chain only ends that chain, and the
// packet still goes through ours.
func (nftBackend) Apply(ns *NetNS, st State) (Applied, error) {
	c, err := nftables.New(nftables.WithNetNSFd(ns.Fd()))
	if err != nil {
		return Applied{}, err
	}
	defer func() { _ = c.CloseLasting() }()

	// The markers alone prove nothing: a verdict may have been changed with the UserData kept.
	cur, curChain, rules := nftGetTable(c), nftGetChain(c), nftGetRules(c)
	if nftIntact(cur, curChain, rules, st) {
		return Applied{}, nil
	}
	res := Applied{Repaired: nftRepaired(cur, curChain, rules, st)}

	t := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}
	// Recreate atomically: add (no-op if exists), delete, add again.
	c.AddTable(t)
	c.DelTable(t)
	c.AddTable(t)
	ch := c.AddChain(nftChain(t))
	// The counters are carried over, so a rewrite does not lose matched packets in the metric.
	counters := nftCounters(rules)
	for _, r := range nftRules(st) {
		name, _, _ := parseMarker(string(r.UserData))
		for _, e := range r.Exprs {
			if cnt, ok := e.(*expr.Counter); ok {
				*cnt = counters[name]
			}
		}
		r.Table, r.Chain = t, ch
		c.AddRule(r)
	}
	if err := c.Flush(); err != nil {
		return res, err
	}
	if back := nftGetRules(c); nftShape(nftGetTable(c), nftGetChain(c), back, st) {
		rememberRendering(nftRenderingKey(st.Mode), nftRendering(back))
	}
	return res, nil
}

func (nftBackend) Read(ns *NetNS) (State, bool) {
	c, err := nftables.New(nftables.WithNetNSFd(ns.Fd()))
	if err != nil {
		return State{}, false
	}
	defer func() { _ = c.CloseLasting() }()
	return nftReadState(c)
}

func (nftBackend) Remove(ns *NetNS) (State, bool, error) {
	c, err := nftables.New(nftables.WithNetNSFd(ns.Fd()))
	if err != nil {
		return State{}, false, err
	}
	defer func() { _ = c.CloseLasting() }()

	tables, err := c.ListTablesOfFamily(nftables.TableFamilyIPv4)
	if err != nil {
		return State{}, false, err
	}
	exists := false
	for _, t := range tables {
		exists = exists || t.Name == tableName
	}
	if !exists {
		return State{}, false, nil
	}
	// The table is removed whatever it contains (edited by hand, written by another guard
	// version): Disabled must not leave a drop rule behind. The MPTCP value is taken from any
	// readable marker.
	st := State{MPTCPOrig: mptcpUnmanaged}
	for _, r := range nftGetRules(c) {
		if _, s, ok := parseMarker(string(r.UserData)); ok {
			st = s
			break
		}
	}
	c.DelTable(&nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4})
	return st, true, c.Flush()
}

func (nftBackend) Counters(ns *NetNS) (map[string]uint64, error) {
	c, err := nftables.New(nftables.WithNetNSFd(ns.Fd()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.CloseLasting() }()

	rules := nftGetRules(c)
	if rules == nil {
		return nil, nil
	}
	out := map[string]uint64{}
	for name, cnt := range nftCounters(rules) {
		out[name] = cnt.Packets
	}
	return out, nil
}

// nftCounters returns the counter of each rule by its marker name.
func nftCounters(rules []*nftables.Rule) map[string]expr.Counter {
	out := map[string]expr.Counter{}
	for _, r := range rules {
		name, _, ok := parseMarker(string(r.UserData))
		if !ok {
			continue
		}
		for _, e := range r.Exprs {
			if cnt, ok := e.(*expr.Counter); ok {
				sum := out[name]
				sum.Packets += cnt.Packets
				sum.Bytes += cnt.Bytes
				out[name] = sum
			}
		}
	}
	return out
}

func nftChain(t *nftables.Table) *nftables.Chain {
	prio := nftables.ChainPriority(chainPriority)
	return &nftables.Chain{
		Name:     chainName,
		Table:    t,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: &prio,
	}
}

// nftGetTable returns nil when the table does not exist.
func nftGetTable(c *nftables.Conn) *nftables.Table {
	t, err := c.ListTableOfFamily(tableName, nftables.TableFamilyIPv4)
	if err != nil {
		return nil
	}
	return t
}

// nftGetChain returns nil when the table or the chain does not exist.
func nftGetChain(c *nftables.Conn) *nftables.Chain {
	ch, err := c.ListChain(&nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}, chainName)
	if err != nil {
		return nil
	}
	return ch
}

// nftGetRules returns nil when the table or the chain does not exist.
func nftGetRules(c *nftables.Conn) []*nftables.Rule {
	t := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}
	rules, err := c.GetRules(t, &nftables.Chain{Name: chainName, Table: t})
	if err != nil {
		return nil
	}
	return rules
}

func nftReadState(c *nftables.Conn) (State, bool) {
	return nftParseState(nftGetRules(c))
}

func nftParseState(rules []*nftables.Rule) (State, bool) {
	if len(rules) != len(ruleNames) {
		return State{}, false
	}
	var st State
	for i, r := range rules {
		name, s, ok := parseMarker(string(r.UserData))
		if !ok || name != ruleNames[i] || (i > 0 && s != st) {
			return State{}, false
		}
		st = s
	}
	return st, true
}

// nftIntact reports whether nftShape holds and the rules match and drop exactly as written: as
// nftRules builds them, or as the kernel listed them right after the guard wrote them.
func nftIntact(t *nftables.Table, ch *nftables.Chain, rules []*nftables.Rule, st State) bool {
	if !nftShape(t, ch, rules, st) {
		return false
	}
	if knownRendering(nftRenderingKey(st.Mode), nftRendering(rules)) {
		return true
	}
	for i, want := range nftRules(st) {
		if !exprsMatch(rules[i].Exprs, want.Exprs) {
			return false
		}
	}
	return true
}

// nftRepaired reports whether a rewrite of the existing filter repairs a change made outside the
// guard, rather than installs it or changes its mode.
func nftRepaired(t *nftables.Table, ch *nftables.Chain, rules []*nftables.Rule, st State) bool {
	if prev, ok := nftParseState(rules); t == nil || (ok && prev != st) {
		return false
	}
	if !nftShape(t, ch, rules, st) {
		return true
	}
	// Only the listing of the matches differs. Right after a restart it is not known how this kernel
	// lists our own filter, so a different listing is not taken for a change.
	return hasRendering(nftRenderingKey(st.Mode))
}

// nftShape reports whether the table has no flags (dormant turns its chains off), the chain is
// the prerouting base chain and the rules carry the markers and the final verdict of st.
func nftShape(t *nftables.Table, ch *nftables.Chain, rules []*nftables.Rule, st State) bool {
	// Any flag counts: google/nftables v0.3.0 decodes them in host byte order, dormant reads as 1<<24 here.
	if t == nil || t.Flags != 0 {
		return false
	}
	want := nftChain(nil)
	if ch == nil || ch.Type != want.Type || ch.Hooknum == nil || *ch.Hooknum != *want.Hooknum ||
		ch.Priority == nil || *ch.Priority != *want.Priority {
		return false
	}
	if cur, ok := nftParseState(rules); !ok || cur != st {
		return false
	}
	for _, r := range rules {
		if len(r.Exprs) == 0 {
			return false
		}
		last := r.Exprs[len(r.Exprs)-1]
		if v, ok := last.(*expr.Verdict); st.Mode == ModeEnforce && (!ok || v.Kind != expr.VerdictDrop) {
			return false
		}
		if _, ok := last.(*expr.Counter); st.Mode == ModeAudit && !ok {
			return false
		}
	}
	return true
}

func nftRenderingKey(mode Mode) string { return "nftables/" + string(mode) }

// nftRendering prints the expressions of the rules; counter values are left out.
func nftRendering(rules []*nftables.Rule) string {
	var b strings.Builder
	for _, r := range rules {
		for _, e := range r.Exprs {
			if _, ok := e.(*expr.Counter); ok {
				b.WriteString("counter ")
				continue
			}
			fmt.Fprintf(&b, "%T%+v ", e, e)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// exprsMatch compares the expressions field by field; only counter values are ignored.
func exprsMatch(got, want []expr.Any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if _, ok := want[i].(*expr.Counter); ok {
			if _, ok := got[i].(*expr.Counter); !ok {
				return false
			}
			continue
		}
		if !reflect.DeepEqual(got[i], want[i]) {
			return false
		}
	}
	return true
}

// IHL > 5, i.e. the IPv4 header carries options.
func ihlGt5() []expr.Any {
	return []expr.Any{
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 0, Len: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x0f}, Xor: []byte{0x00}},
		&expr.Cmp{Op: expr.CmpOpGt, Register: 1, Data: []byte{5}},
	}
}

func verdict(st State) []expr.Any {
	if st.Mode == ModeAudit {
		return []expr.Any{&expr.Counter{}}
	}
	return []expr.Any{&expr.Counter{}, &expr.Verdict{Kind: expr.VerdictDrop}}
}

func nftRules(st State) []*nftables.Rule {
	syn := make([]expr.Any, 0, 10)
	syn = append(syn,
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 9, Len: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{6}},
	)
	syn = append(syn, ihlGt5()...)
	syn = append(syn,
		// TCP flags: SYN set, ACK unset.
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseTransportHeader, Offset: 13, Len: 1},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 1, Mask: []byte{0x12}, Xor: []byte{0x00}},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{0x02}},
	)
	syn = append(syn, verdict(st)...)

	// Fragments carrying options: covers netns without conntrack, where packets are not reassembled before this hook.
	frag := ihlGt5()
	frag = append(frag,
		&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 6, Len: 2},
		&expr.Bitwise{SourceRegister: 1, DestRegister: 1, Len: 2, Mask: []byte{0x3f, 0xff}, Xor: []byte{0x00, 0x00}},
		&expr.Cmp{Op: expr.CmpOpNeq, Register: 1, Data: []byte{0x00, 0x00}},
	)
	frag = append(frag, verdict(st)...)

	return []*nftables.Rule{
		{Exprs: syn, UserData: []byte(marker(ruleNames[0], st))},
		{Exprs: frag, UserData: []byte(marker(ruleNames[1], st))},
	}
}
