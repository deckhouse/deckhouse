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
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

const (
	legacyChain = "D8-IPV4OPT-GUARD"
	// The new filter is built here and renamed to legacyChain once PREROUTING jumps to it.
	legacyChainNew = legacyChain + "-NEW"
	// State is kept in the name of an empty marker chain, so xt_comment is not required.
	legacyStatePrefix = "D8-IPV4OPT:"
	// IHL > 5 is only expressible via u32 in iptables.
	u32IHL  = "0>>22&0x3C=24:60"
	u32Frag = u32IHL + "&&4&0x3FFF=1:0x3FFF"
)

// legacyBackend uses iptables-legacy with the u32 match; selected only where nf_tables is not loaded.
type legacyBackend struct {
	bin string
}

func (legacyBackend) Name() string { return "iptables-legacy" }

func (b legacyBackend) run(args ...string) (string, error) {
	out, err := exec.Command(b.bin, append([]string{"-w", "-t", "raw"}, args...)...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w: %s", b.bin, strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return string(out), nil
}

func stateChain(st State) string {
	return legacyStatePrefix + string(st.Mode[0]) + ":" + st.MPTCPOrig
}

func (b legacyBackend) readState() (State, bool) {
	out, err := b.run("-S")
	if err != nil {
		return State{}, false
	}
	return parseLegacyState(out)
}

// parseLegacyState reads the marker chain. Two markers are left by an interrupted switch; the one
// with a recorded MPTCP value wins, as the value was already changed when it was created.
func parseLegacyState(rules string) (State, bool) {
	var st State
	found := false
	sc := bufio.NewScanner(strings.NewReader(rules))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 2 || f[0] != "-N" || !strings.HasPrefix(f[1], legacyStatePrefix) {
			continue
		}
		m, mptcp, ok := strings.Cut(strings.TrimPrefix(f[1], legacyStatePrefix), ":")
		mode := map[string]Mode{"E": ModeEnforce, "A": ModeAudit}[m]
		if !ok || mode == "" {
			continue
		}
		if !found || (st.MPTCPOrig == mptcpUnmanaged && mptcp != mptcpUnmanaged) {
			st, found = State{Mode: mode, MPTCPOrig: mptcp}, true
		}
	}
	return st, found
}

func (b legacyBackend) Apply(ns *NetNS, st State) (Applied, error) {
	var res Applied
	err := ns.Do(func() error {
		out, err := b.run("-S")
		if err != nil {
			return err
		}
		// The marker chain alone proves nothing: the rules or the jump may have been flushed or changed.
		cur, ok := parseLegacyState(out)
		if ok && cur == st && legacyIntact(out, st.Mode) {
			res.Bypassed = legacyBypassed(out)
			return nil
		}
		exists := slices.Contains(strings.Split(out, "\n"), "-N "+legacyChain)
		res.Repaired = legacyRepaired(out, st)
		// The counters are carried over, so a rewrite does not lose matched packets in the metric.
		var counters [][2]uint64
		if exists {
			if res, err := b.run("-L", legacyChain, "-n", "-v", "-x"); err == nil {
				counters = parseLegacyCounters(res)
			}
		}
		withCounters := func(i int, args []string) []string {
			if i < len(counters) {
				args = append(args, "-c", strconv.FormatUint(counters[i][0], 10), strconv.FormatUint(counters[i][1], 10))
			}
			return args
		}
		// An earlier switch stopped after removing the old filter: finish it instead of dropping the only one.
		if !hasJump(out, legacyChain) && hasJump(out, legacyChainNew) {
			if err := b.flush([]string{legacyChain}, false, ""); err != nil {
				return err
			}
			if _, err := b.run("-E", legacyChainNew, legacyChain); err != nil {
				return err
			}
		}
		// Build the new filter next to the old one and switch the jump before removing the old
		// one, so the netns is never left unfiltered.
		if err := b.flush([]string{legacyChainNew}, false, ""); err != nil {
			return err
		}
		// The jump is put first on install only. A rewrite keeps its place, so a rule another tool
		// put ahead of it stays ahead: it is reported as Bypassed, not fought with.
		pos := "1"
		if out, err := b.run("-S"); err != nil {
			return err
		} else if i := jumpPosition(out, legacyChain); i > 0 {
			pos = strconv.Itoa(i)
		}
		target := []string{"-j", "DROP"}
		if st.Mode == ModeAudit {
			target = nil
		}
		cmds := [][]string{
			{"-N", legacyChainNew},
			withCounters(0, append([]string{"-A", legacyChainNew, "-p", "tcp", "--syn", "-m", "u32", "--u32", u32IHL}, target...)),
			withCounters(1, append([]string{"-A", legacyChainNew, "-m", "u32", "--u32", u32Frag}, target...)),
			{"-I", "PREROUTING", pos, "-j", legacyChainNew},
		}
		for _, c := range cmds {
			if _, err := b.run(c...); err != nil {
				return err
			}
		}
		// The new marker is created before the old ones go, so the original MPTCP value is never lost.
		marker := stateChain(st)
		if out, err := b.run("-S"); err != nil {
			return err
		} else if !slices.Contains(strings.Split(out, "\n"), "-N "+marker) {
			if _, err := b.run("-N", marker); err != nil {
				return err
			}
		}
		if err := b.flush([]string{legacyChain}, true, marker); err != nil {
			return err
		}
		// The rename keeps the PREROUTING jump.
		if _, err := b.run("-E", legacyChainNew, legacyChain); err != nil {
			return err
		}
		if out, err := b.run("-S"); err == nil {
			if legacyShape(out, st.Mode) {
				rememberRendering(legacyRenderingKey(st.Mode), legacyRendering(out))
			}
			res.Bypassed = legacyBypassed(out)
		}
		return nil
	})
	return res, err
}

func legacyRenderingKey(mode Mode) string { return "iptables-legacy/" + string(mode) }

// legacyRendering returns the rules of the filter chain as `iptables -S` prints them.
func legacyRendering(rules string) string {
	var chain []string
	for _, line := range strings.Split(rules, "\n") {
		if strings.HasPrefix(line, "-A "+legacyChain+" ") {
			chain = append(chain, line)
		}
	}
	return strings.Join(chain, "\n")
}

// legacyRules are the filter rules as `iptables -S` prints them, without the target.
var legacyRules = []string{
	"-A " + legacyChain + ` -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN -m u32 --u32 "0x0>>0x16&0x3c=0x18:0x3c"`,
	"-A " + legacyChain + ` -m u32 --u32 "0x0>>0x16&0x3c=0x18:0x3c&&0x4&0x3fff=0x1:0x3fff"`,
}

// legacyRepaired reports whether a rewrite of the existing filter repairs a change made outside
// the guard, rather than installs it or changes its mode.
func legacyRepaired(rules string, st State) bool {
	cur, ok := parseLegacyState(rules)
	if !slices.Contains(strings.Split(rules, "\n"), "-N "+legacyChain) || (ok && cur != st) {
		return false
	}
	if !ok || !legacyShape(rules, st.Mode) {
		return true
	}
	// Only the listing of the matches differs; see nftRepaired.
	return hasRendering(legacyRenderingKey(st.Mode))
}

// legacyIntact reports whether legacyShape holds and the rules are exactly the ones of the mode:
// as legacyRules lists them, or as `iptables -S` printed them right after the guard wrote them.
func legacyIntact(rules string, mode Mode) bool {
	if !legacyShape(rules, mode) {
		return false
	}
	chain := legacyRendering(rules)
	if knownRendering(legacyRenderingKey(mode), chain) {
		return true
	}
	target := ""
	if mode == ModeEnforce {
		target = " -j DROP"
	}
	for i, line := range strings.Split(chain, "\n") {
		if line != legacyRules[i]+target {
			return false
		}
	}
	return true
}

// legacyShape reports whether PREROUTING jumps to the filter chain exactly once, the chain has as
// many rules as legacyRules with the target of the mode (Enforce drops, Audit only counts), and
// nothing is left of an interrupted switch (the -NEW chain, a second marker).
func legacyShape(rules string, mode Mode) bool {
	jumps, markers := 0, 0
	var chain []string
	sc := bufio.NewScanner(strings.NewReader(rules))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "-A PREROUTING -j "+legacyChain:
			jumps++
		case strings.HasPrefix(line, "-A "+legacyChain+" "):
			chain = append(chain, line)
		case line == "-N "+legacyChainNew:
			return false
		case strings.HasPrefix(line, "-N "+legacyStatePrefix):
			markers++
		}
	}
	if jumps != 1 || markers != 1 || len(chain) != len(legacyRules) {
		return false
	}
	for _, line := range chain {
		drops := strings.HasSuffix(line, " -j DROP")
		if (mode == ModeEnforce) != drops || (!drops && (strings.Contains(line, " -j ") || strings.Contains(line, " -g "))) {
			return false
		}
	}
	return true
}

// legacyBypassed reports whether a rule in PREROUTING ahead of the jump to the filter can end the
// traversal of the raw table, so packets it accepts never reach the filter.
func legacyBypassed(rules string) bool {
	var before []string
	chains := map[string][]string{}
	jumped := false
	for _, line := range strings.Split(rules, "\n") {
		switch {
		case line == "-A PREROUTING -j "+legacyChain:
			jumped = true
		case strings.HasPrefix(line, "-A PREROUTING "):
			if !jumped {
				before = append(before, line)
			}
		case strings.HasPrefix(line, "-N "):
			if name := strings.TrimPrefix(line, "-N "); chains[name] == nil {
				chains[name] = []string{}
			}
		case strings.HasPrefix(line, "-A "):
			if name, _, ok := strings.Cut(strings.TrimPrefix(line, "-A "), " "); ok {
				chains[name] = append(chains[name], line)
			}
		}
	}
	return jumped && !passesOn(before, chains, false, 0)
}

// jumpPosition returns the 1-based position of the jump to chain in PREROUTING, or 0.
func jumpPosition(rules, chain string) int {
	i := 0
	for _, line := range strings.Split(rules, "\n") {
		if strings.HasPrefix(line, "-A PREROUTING ") {
			i++
			if line == "-A PREROUTING -j "+chain {
				return i
			}
		}
	}
	return 0
}

// nonTerminal are the targets after which a packet goes on to the next rule.
var nonTerminal = map[string]bool{
	"CT": true, "NOTRACK": true, "MARK": true, "CONNMARK": true, "HMARK": true, "LOG": true, "NFLOG": true,
	"TRACE": true, "AUDIT": true, "SET": true, "TEE": true, "IDLETIMER": true, "RATEEST": true,
}

// passesOn reports whether no packet can get past the rules without going on to the next rule:
// a DROP is fine, it lets nothing through. Other agents (cilium) insert their raw chains first
// too, so jumps are followed. A RETURN or a goto only ends a nested chain, not PREROUTING;
// anything else not known to pass on is taken as an accepting verdict.
func passesOn(rules []string, chains map[string][]string, nested bool, depth int) bool {
	if depth > 8 {
		return false
	}
	for _, line := range rules {
		kind, target := ruleTarget(line)
		sub, isChain := chains[target]
		switch {
		case kind == "", kind == "-j" && (nonTerminal[target] || target == "DROP"), kind == "-j" && nested && target == "RETURN":
		case isChain && (kind == "-j" || nested):
			if !passesOn(sub, chains, true, depth+1) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ruleTarget returns the last -j or -g of a rule as `iptables -S` prints it and its target.
func ruleTarget(line string) (string, string) {
	f := strings.Fields(line)
	for i := len(f) - 2; i >= 0; i-- {
		if f[i] == "-j" || f[i] == "-g" {
			return f[i], f[i+1]
		}
	}
	return "", ""
}

func hasJump(rules, chain string) bool {
	for _, line := range strings.Split(rules, "\n") {
		if line == "-A PREROUTING -j "+chain {
			return true
		}
	}
	return false
}

// flush removes the chains with their PREROUTING jumps and, with state set, the marker chains except keep.
func (b legacyBackend) flush(chains []string, state bool, keep string) error {
	out, err := b.run("-S")
	if err != nil {
		return err
	}
	for _, c := range chains {
		for hasJump(out, c) {
			if _, err := b.run("-D", "PREROUTING", "-j", c); err != nil {
				return err
			}
			if out, err = b.run("-S"); err != nil {
				return err
			}
		}
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && f[0] == "-N" && (slices.Contains(chains, f[1]) || (state && f[1] != keep && strings.HasPrefix(f[1], legacyStatePrefix))) {
			_, _ = b.run("-F", f[1])
			if _, err := b.run("-X", f[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// rawTable reports whether the raw table exists in the netns of the calling thread. Any iptables
// call on it would create it and register its hooks, e.g. in every pod netns of an nft node
// while looking for a filter left by an earlier legacy run.
func rawTable() bool {
	b, err := os.ReadFile("/proc/thread-self/net/ip_tables_names")
	return err == nil && slices.Contains(strings.Fields(string(b)), "raw")
}

func (b legacyBackend) Read(ns *NetNS) (State, bool) {
	var st State
	var found bool
	_ = ns.Do(func() error {
		if !rawTable() {
			return nil
		}
		st, found = b.readState()
		return nil
	})
	return st, found
}

func (b legacyBackend) Remove(ns *NetNS) (State, bool, error) {
	var st State
	var found bool
	err := ns.Do(func() error {
		if !rawTable() {
			return nil
		}
		st, found = b.readState()
		return b.flush([]string{legacyChain, legacyChainNew}, true, "")
	})
	return st, found, err
}

func (b legacyBackend) Counters(ns *NetNS) (map[string]uint64, error) {
	out := map[string]uint64{}
	err := ns.Do(func() error {
		res, err := b.run("-L", legacyChain, "-n", "-v", "-x")
		if err != nil {
			return nil // chain absent
		}
		for i, c := range parseLegacyCounters(res) {
			if i < len(ruleNames) {
				out[ruleNames[i]] += c[0]
			}
		}
		return nil
	})
	return out, err
}

// parseLegacyCounters returns packets and bytes of each rule of `iptables -L <chain> -n -v -x`.
func parseLegacyCounters(res string) [][2]uint64 {
	var out [][2]uint64
	sc := bufio.NewScanner(strings.NewReader(res))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		pkts, err1 := strconv.ParseUint(f[0], 10, 64)
		bytes, err2 := strconv.ParseUint(f[1], 10, 64)
		if err1 != nil || err2 != nil {
			continue // header lines
		}
		out = append(out, [2]uint64{pkts, bytes})
	}
	return out
}
