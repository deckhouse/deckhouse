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
	"strings"
	"testing"
)

func TestHasJumpExactChain(t *testing.T) {
	rules := "-P PREROUTING ACCEPT\n-N " + legacyChainNew + "\n-A PREROUTING -j " + legacyChainNew + "\n"
	if !hasJump(rules, legacyChainNew) {
		t.Fatal("jump to the new chain not found")
	}
	// legacyChain is a prefix of legacyChainNew: it must not match.
	if hasJump(rules, legacyChain) {
		t.Fatal("jump to the new chain taken for a jump to the old one")
	}
}

func TestParseLegacyStateTwoMarkers(t *testing.T) {
	// Audit without MPTCP changes -> Enforce interrupted after the new marker was created.
	rules := "-N D8-IPV4OPT:A:" + mptcpUnmanaged + "\n-N D8-IPV4OPT:E:1\n"
	st, ok := parseLegacyState(rules)
	if !ok || st.MPTCPOrig != "1" || st.Mode != ModeEnforce {
		t.Fatalf("got %+v, %v; want the marker with the original MPTCP value", st, ok)
	}
	if _, ok := parseLegacyState("-N D8-IPV4OPT-GUARD\n"); ok {
		t.Fatal("state found without a marker")
	}
}

func TestLegacyIntact(t *testing.T) {
	forgetRenderings()
	full := "-P PREROUTING ACCEPT\n-N D8-IPV4OPT-GUARD\n-N D8-IPV4OPT:E:1\n" +
		"-A PREROUTING -j D8-IPV4OPT-GUARD\n" +
		"-A D8-IPV4OPT-GUARD -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN -m u32 --u32 \"0x0>>0x16&0x3c=0x18:0x3c\" -j DROP\n" +
		"-A D8-IPV4OPT-GUARD -m u32 --u32 \"0x0>>0x16&0x3c=0x18:0x3c&&0x4&0x3fff=0x1:0x3fff\" -j DROP\n"
	if !legacyIntact(full, ModeEnforce) {
		t.Fatal("complete filter reported as broken")
	}
	if legacyIntact(full, ModeAudit) {
		t.Fatal("dropping filter reported as intact in Audit")
	}
	if audit := strings.ReplaceAll(full, " -j DROP", ""); !legacyIntact(audit, ModeAudit) {
		t.Fatal("counting filter reported as broken in Audit")
	}
	noJump := strings.Replace(full, "-A PREROUTING -j D8-IPV4OPT-GUARD\n", "", 1)
	flushed := "-P PREROUTING ACCEPT\n-N D8-IPV4OPT-GUARD\n-N D8-IPV4OPT:E:1\n-A PREROUTING -j D8-IPV4OPT-GUARD\n"
	doubleJump := full + "-A PREROUTING -j D8-IPV4OPT-GUARD\n"
	// Leftovers of an interrupted switch.
	newChain := full + "-N D8-IPV4OPT-GUARD-NEW\n-A PREROUTING -j D8-IPV4OPT-GUARD-NEW\n"
	twoMarkers := full + "-N D8-IPV4OPT:A:1\n"
	noMarker := strings.Replace(full, "-N D8-IPV4OPT:E:1\n", "", 1)
	for name, rules := range map[string]string{
		"no jump": noJump, "flushed chain": flushed, "double jump": doubleJump,
		"new chain": newChain, "two markers": twoMarkers, "no marker": noMarker,
		"accept":            strings.Replace(full, "-j DROP\n", "-j ACCEPT\n", 1),
		"no target":         strings.Replace(full, " -j DROP\n", "\n", 1),
		"goto":              strings.Replace(full, " -j DROP\n", " -g OTHER\n", 1),
		"u32 never matches": strings.Replace(full, "=0x18:0x3c\" -j", "=0x0:0x0\" -j", 1),
		"rules mixed up": strings.Replace(strings.Replace(full, "-p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN ", "", 1),
			"&&0x4&0x3fff=0x1:0x3fff", "", 1),
	} {
		if legacyIntact(rules, ModeEnforce) {
			t.Fatalf("%s reported as intact", name)
		}
	}
}

func TestLegacyIntactReadBack(t *testing.T) {
	forgetRenderings()
	t.Cleanup(forgetRenderings)
	// Another iptables version prints the match differently.
	listed := "-N D8-IPV4OPT-GUARD\n-N D8-IPV4OPT:E:1\n-A PREROUTING -j D8-IPV4OPT-GUARD\n" +
		"-A D8-IPV4OPT-GUARD -p tcp -m tcp --tcp-flags 0x17 0x02 -m u32 --u32 \"0x0>>0x16&0x3c=0x18:0x3c\" -j DROP\n" +
		"-A D8-IPV4OPT-GUARD -m u32 --u32 \"0x0>>0x16&0x3c=0x18:0x3c&&0x4&0x3fff=0x1:0x3fff\" -j DROP\n"
	if legacyIntact(listed, ModeEnforce) {
		t.Fatal("a different listing is intact before it was read back")
	}
	rememberRendering(legacyRenderingKey(ModeEnforce), legacyRendering(listed))
	if !legacyIntact(listed, ModeEnforce) {
		t.Fatal("the listing read back after the write is not intact")
	}
	if legacyIntact(strings.Replace(listed, "-j DROP\n", "-j ACCEPT\n", 1), ModeEnforce) {
		t.Fatal("a changed target is intact")
	}
	if legacyIntact(strings.Replace(listed, "-A PREROUTING -j D8-IPV4OPT-GUARD\n", "", 1), ModeEnforce) {
		t.Fatal("the read-back listing is intact without the jump")
	}
}

func TestParseLegacyCounters(t *testing.T) {
	res := "Chain D8-IPV4OPT-GUARD (1 references)\n" +
		"    pkts      bytes target     prot opt in     out     source               destination\n" +
		"       5      300 DROP       6    --  *      *       0.0.0.0/0            0.0.0.0/0            tcp flags:0x17/0x02 u32 \"0x0>>0x16&0x3c=0x18:0x3c\"\n" +
		"       0        0 DROP       0    --  *      *       0.0.0.0/0            0.0.0.0/0            u32 \"0x0>>0x16&0x3c=0x18:0x3c&&0x4&0x3fff=0x1:0x3fff\"\n"
	got := parseLegacyCounters(res)
	if len(got) != 2 || got[0] != [2]uint64{5, 300} || got[1] != [2]uint64{0, 0} {
		t.Fatalf("got %v", got)
	}
}

func TestLegacyRepaired(t *testing.T) {
	forgetRenderings()
	t.Cleanup(forgetRenderings)
	st := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	full := "-N D8-IPV4OPT-GUARD\n-N D8-IPV4OPT:E:1\n-A PREROUTING -j D8-IPV4OPT-GUARD\n" +
		legacyRules[0] + " -j DROP\n" + legacyRules[1] + " -j DROP\n"
	// Another iptables version prints the tcp match differently.
	listed := strings.Replace(full, "FIN,SYN,RST,ACK SYN", "0x17 0x02", 1)

	if legacyRepaired("", st) {
		t.Error("first install reported as a repair")
	}
	if legacyRepaired(strings.Replace(full, ":E:", ":A:", 1), st) {
		t.Error("mode change reported as a repair")
	}
	for name, rules := range map[string]string{
		"accept":    strings.Replace(full, "-j DROP\n", "-j ACCEPT\n", 1),
		"flushed":   "-N D8-IPV4OPT-GUARD\n-N D8-IPV4OPT:E:1\n-A PREROUTING -j D8-IPV4OPT-GUARD\n",
		"no jump":   strings.Replace(full, "-A PREROUTING -j D8-IPV4OPT-GUARD\n", "", 1),
		"no marker": strings.Replace(full, "-N D8-IPV4OPT:E:1\n", "", 1),
	} {
		if !legacyRepaired(rules, st) {
			t.Errorf("%s not reported", name)
		}
	}
	if legacyRepaired(listed, st) {
		t.Error("a different listing reported as a repair after a restart")
	}
	rememberRendering(legacyRenderingKey(st.Mode), legacyRendering(full))
	if !legacyRepaired(listed, st) {
		t.Error("changed match not reported once our listing is known")
	}
}

func TestLegacyBypassed(t *testing.T) {
	forgetRenderings()
	filter := "-A PREROUTING -j D8-IPV4OPT-GUARD\n" +
		"-A D8-IPV4OPT-GUARD -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN -m u32 --u32 \"0x0>>0x16&0x3c=0x18:0x3c\" -j DROP\n" +
		"-A D8-IPV4OPT-GUARD -m u32 --u32 \"0x0>>0x16&0x3c=0x18:0x3c&&0x4&0x3fff=0x1:0x3fff\" -j DROP\n"
	chains := "-P PREROUTING ACCEPT\n-N CILIUM_PRE_raw\n-N D8-IPV4OPT-GUARD\n-N D8-IPV4OPT:E:1\n-N OTHER\n"
	cilium := "-A PREROUTING -m comment --comment \"cilium-feeder: CILIUM_PRE_raw\" -j CILIUM_PRE_raw\n" +
		"-A CILIUM_PRE_raw -m mark --mark 0x200/0xf00 -m comment --comment \"cilium: NOTRACK for proxy traffic\" -j CT --notrack\n"

	for name, ahead := range map[string]string{
		"cilium raw chain": cilium,
		"notrack":          "-A PREROUTING -d 169.254.20.10/32 -m socket --nowildcard -j NOTRACK\n",
		"counting rule":    "-A PREROUTING -p tcp\n",
		"return in chain":  "-A PREROUTING -j OTHER\n-A OTHER -s 10.0.0.0/8 -j RETURN\n-A OTHER -j MARK --set-xmark 0x1/0x1\n",
		"empty chain":      "-A PREROUTING -j OTHER\n",
		"ipset blacklist":  "-A PREROUTING -m set --match-set blacklist src -j DROP\n",
		"drop in chain":    "-A PREROUTING -j OTHER\n-A OTHER -s 192.0.2.0/24 -j DROP\n",
	} {
		if legacyBypassed(chains + ahead + filter) {
			t.Errorf("%s ahead of the jump: reported as a bypass", name)
		}
	}
	if legacyBypassed(chains + filter + "-A PREROUTING -j ACCEPT\n") {
		t.Error("an ACCEPT after the jump: reported as a bypass")
	}

	for name, ahead := range map[string]string{
		"accept":            "-A PREROUTING -j ACCEPT\n",
		"return":            "-A PREROUTING -p tcp -j RETURN\n",
		"goto":              "-A PREROUTING -g OTHER\n",
		"accept in chain":   "-A PREROUTING -j OTHER\n-A OTHER -s 10.0.0.0/8 -j ACCEPT\n",
		"accept in cilium":  cilium + "-A CILIUM_PRE_raw -j ACCEPT\n",
		"unknown target":    "-A PREROUTING -j SOMETHING\n",
		"verdict in prefix": "-A PREROUTING -j LOG --log-prefix \"x -j ACCEPT\"\n",
		"loop":              "-A PREROUTING -j OTHER\n-A OTHER -j OTHER\n",
	} {
		rules := chains + ahead + filter
		if !legacyBypassed(rules) {
			t.Errorf("%s ahead of the jump: not reported as a bypass", name)
		}
		// The filter itself is intact: the guard does not rewrite it to move the jump.
		if !legacyIntact(rules, ModeEnforce) {
			t.Errorf("%s ahead of the jump: the filter is reported as broken", name)
		}
	}
	if legacyBypassed(chains) {
		t.Error("no filter: reported as a bypass")
	}
}

func TestJumpPosition(t *testing.T) {
	rules := "-N D8-IPV4OPT-GUARD\n-A PREROUTING -j CILIUM_PRE_raw\n-A PREROUTING -j ACCEPT\n-A PREROUTING -j D8-IPV4OPT-GUARD\n"
	if got := jumpPosition(rules, legacyChain); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
	if got := jumpPosition(rules, legacyChainNew); got != 0 {
		t.Fatalf("got %d for a missing jump", got)
	}
}
