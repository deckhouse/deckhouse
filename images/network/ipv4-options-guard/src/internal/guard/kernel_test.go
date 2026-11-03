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
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

// Tests against the real kernel, in a throwaway netns; run as root with GUARD_KERNEL_TESTS=1:
// docker run --rm --privileged -e GUARD_KERNEL_TESTS=1 -v "$PWD":/src -w /src golang:1.26 go test ./...
func newTestNetNS(t *testing.T) *NetNS {
	if os.Getenv("GUARD_KERNEL_TESTS") != "1" {
		t.Skip("GUARD_KERNEL_TESTS is not set")
	}
	type res struct {
		ns  *NetNS
		err error
	}
	ch := make(chan res, 1)
	go func() {
		// The thread is not unlocked: it stays in the new netns and exits with the goroutine.
		runtime.LockOSThread()
		if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
			ch <- res{err: err}
			return
		}
		ns, err := OpenNetNS("test", "/proc/thread-self/ns/net")
		ch <- res{ns, err}
	}()
	r := <-ch
	if r.err != nil {
		t.Fatal(r.err)
	}
	t.Cleanup(func() { _ = r.ns.Close() })
	return r.ns
}

func TestKernelNftIntact(t *testing.T) {
	ns := newTestNetNS(t)
	b := nftBackend{}
	enforce := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	audit := State{Mode: ModeAudit, MPTCPOrig: mptcpUnmanaged}

	intact := func(st State) bool {
		c, err := nftables.New(nftables.WithNetNSFd(ns.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.CloseLasting() }()
		return nftIntact(nftGetTable(c), nftGetChain(c), nftGetRules(c), st)
	}
	for _, st := range []State{enforce, audit} {
		if _, err := b.Apply(ns, st); err != nil {
			t.Fatal(err)
		}
		// The kernel dump must match the expected form even without the read-back.
		forgetRenderings()
		if !intact(st) {
			t.Fatalf("%s: the applied filter is not recognized as intact", st.Mode)
		}
	}
	if intact(State{Mode: ModeEnforce, MPTCPOrig: mptcpUnmanaged}) {
		t.Fatal("the counting filter is taken for the dropping one")
	}

	// A dormant table keeps the rules but filters nothing; the library cannot set the flag.
	nft, err := exec.LookPath("nft")
	if err != nil {
		t.Skip("nft is not installed")
	}
	if err := ns.Do(func() error {
		return exec.Command(nft, "add table ip "+tableName+" { flags dormant; }").Run()
	}); err != nil {
		t.Fatal(err)
	}
	if intact(audit) {
		t.Fatal("the dormant table is taken for an active one")
	}
	if res, err := b.Apply(ns, audit); err != nil || !res.Repaired || !intact(audit) {
		t.Fatalf("the dormant table is not repaired: %+v, %v", res, err)
	}
}

func TestKernelNftRepair(t *testing.T) {
	ns := newTestNetNS(t)
	b := nftBackend{}
	enforce := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	if _, err := b.Apply(ns, enforce); err != nil {
		t.Fatal(err)
	}

	// Another controller turns DROP into ACCEPT and keeps the markers; the rule has matched 5 packets.
	c, err := nftables.New(nftables.WithNetNSFd(ns.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseLasting() }()
	tb := &nftables.Table{Name: tableName, Family: nftables.TableFamilyIPv4}
	c.DelTable(tb)
	c.AddTable(tb)
	ch := c.AddChain(nftChain(tb))
	for _, r := range nftRules(enforce) {
		for _, e := range r.Exprs {
			switch e := e.(type) {
			case *expr.Verdict:
				e.Kind = expr.VerdictAccept
			case *expr.Counter:
				e.Packets, e.Bytes = 5, 300
			}
		}
		r.Table, r.Chain = tb, ch
		c.AddRule(r)
	}
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}

	if res, err := b.Apply(ns, enforce); err != nil || !res.Repaired {
		t.Fatalf("the changed filter is not repaired: %+v, %v", res, err)
	}
	forgetRenderings()
	if !nftIntact(nftGetTable(c), nftGetChain(c), nftGetRules(c), enforce) {
		t.Fatal("the filter is not intact after the repair")
	}
	if cnt, err := b.Counters(ns); err != nil || cnt[ruleNames[0]] != 5 {
		t.Fatalf("counters are not carried over: %v, %v", cnt, err)
	}
	// A mode change rewrites the filter too, but it is not a repair.
	if res, err := b.Apply(ns, State{Mode: ModeAudit, MPTCPOrig: "1"}); err != nil || res.Repaired {
		t.Fatalf("a mode change is reported as a repair: %+v, %v", res, err)
	}
}

func TestKernelLegacy(t *testing.T) {
	ns := newTestNetNS(t)
	bin, err := exec.LookPath("iptables-legacy")
	if err != nil {
		t.Skip("iptables-legacy is not installed")
	}
	b := legacyBackend{bin: bin}

	// Looking for a leftover filter must not create the raw table.
	if _, found := b.Read(ns); found {
		t.Fatal("state found in an empty netns")
	}
	var raw bool
	_ = ns.Do(func() error { raw = rawTable(); return nil })
	if raw {
		t.Fatal("Read created the raw table")
	}

	for _, st := range []State{{Mode: ModeEnforce, MPTCPOrig: "1"}, {Mode: ModeAudit, MPTCPOrig: "1"}} {
		if _, err := b.Apply(ns, st); err != nil {
			t.Fatal(err)
		}
		// The `-S` output must match the expected form even without the read-back.
		forgetRenderings()
		var out string
		_ = ns.Do(func() error { out, err = b.run("-S"); return err })
		if got, ok := parseLegacyState(out); !ok || got != st || !legacyIntact(out, st.Mode) {
			t.Fatalf("%s: the applied filter is not recognized as intact:\n%s", st.Mode, out)
		}
	}

	// Another controller replaces a rule; it has matched 5 packets.
	audit := State{Mode: ModeAudit, MPTCPOrig: "1"}
	if err := ns.Do(func() error { _, err := b.run("-R", legacyChain, "1", "-c", "5", "300", "-j", "ACCEPT"); return err }); err != nil {
		t.Fatal(err)
	}
	if res, err := b.Apply(ns, audit); err != nil || !res.Repaired {
		t.Fatalf("the changed filter is not repaired: %+v, %v", res, err)
	}
	if cnt, err := b.Counters(ns); err != nil || cnt[ruleNames[0]] != 5 {
		t.Fatalf("counters are not carried over: %v, %v", cnt, err)
	}
	enforce := State{Mode: ModeEnforce, MPTCPOrig: "1"}
	if res, err := b.Apply(ns, enforce); err != nil || res.Repaired {
		t.Fatalf("a mode change is reported as a repair: %+v, %v", res, err)
	}

	// A raw chain inserted ahead of the jump, as cilium does, is passed through.
	insert := func(args ...[]string) {
		if err := ns.Do(func() error {
			for _, a := range args {
				if _, err := b.run(a...); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	insert([]string{"-N", "CILIUM_PRE_raw"},
		[]string{"-A", "CILIUM_PRE_raw", "-m", "mark", "--mark", "0x200/0xf00", "-j", "CT", "--notrack"},
		[]string{"-I", "PREROUTING", "1", "-m", "comment", "--comment", "cilium-feeder: CILIUM_PRE_raw", "-j", "CILIUM_PRE_raw"})
	if res, err := b.Apply(ns, enforce); err != nil || res.Repaired || res.Bypassed {
		t.Fatalf("a non-terminating chain ahead of the jump: %+v, %v", res, err)
	}
	// An ACCEPT ahead of the jump is reported and left in place, by a rewrite too.
	insert([]string{"-I", "PREROUTING", "1", "-j", "ACCEPT"})
	for _, st := range []State{enforce, audit} {
		if res, err := b.Apply(ns, st); err != nil || res.Repaired || !res.Bypassed {
			t.Fatalf("%s: an ACCEPT ahead of the jump: %+v, %v", st.Mode, res, err)
		}
	}
	var out string
	_ = ns.Do(func() error { out, err = b.run("-S"); return err })
	if !strings.Contains(out, "-A PREROUTING -j ACCEPT\n-A PREROUTING -m comment --comment \"cilium-feeder: CILIUM_PRE_raw\" -j CILIUM_PRE_raw\n-A PREROUTING -j "+legacyChain+"\n") {
		t.Fatalf("the rewrite moved the jump:\n%s", out)
	}
	// The fix the alert suggests: a jump inserted first by hand; the guard drops the old one.
	insert([]string{"-I", "PREROUTING", "1", "-j", legacyChain})
	if res, err := b.Apply(ns, audit); err != nil || res.Bypassed {
		t.Fatalf("bypass reported after the jump was put first: %+v, %v", res, err)
	}
	_ = ns.Do(func() error { out, err = b.run("-S"); return err })
	if jumpPosition(out, legacyChain) != 1 || !legacyIntact(out, ModeAudit) {
		t.Fatalf("the jump put first by hand is not kept:\n%s", out)
	}
	insert([]string{"-D", "PREROUTING", "-j", "ACCEPT"})
	if res, err := b.Apply(ns, audit); err != nil || res.Bypassed || res.Repaired {
		t.Fatalf("after the ACCEPT is gone: %+v, %v", res, err)
	}

	if _, _, err := b.Remove(ns); err != nil {
		t.Fatal(err)
	}
	_ = ns.Do(func() error { out, err = b.run("-S"); return err })
	if hasJump(out, legacyChain) || hasJump(out, legacyChainNew) {
		t.Fatalf("filter left after Remove:\n%s", out)
	}
}

func TestKernelModules(t *testing.T) {
	ns := newTestNetNS(t)
	// Installing a filter loads the modules of its backend, built in or not.
	want := slices.Clone(backendModules[IptablesNFT])
	if _, err := (nftBackend{}).Apply(ns, State{Mode: ModeAudit, MPTCPOrig: mptcpUnmanaged}); err != nil {
		t.Fatal(err)
	}
	if bin, err := exec.LookPath("iptables-legacy"); err == nil {
		if _, err := (legacyBackend{bin: bin}).Apply(ns, State{Mode: ModeAudit, MPTCPOrig: mptcpUnmanaged}); err != nil {
			t.Fatal(err)
		}
		want = append(want, backendModules[IptablesLegacy]...)
	}
	available, err := availableModules("/sys/module", "/proc/kallsyms")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range want {
		if !available(m) {
			t.Errorf("%s is in use but not detected", m)
		}
	}
}
