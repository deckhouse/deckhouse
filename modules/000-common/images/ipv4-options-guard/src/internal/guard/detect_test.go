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
	"path/filepath"
	"strings"
	"testing"
)

func loadedSet(names ...string) func(string) bool {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(n string) bool { return set[n] }
}

var allLegacy = []string{"ip_tables", "iptable_raw", "xt_tcpudp", "xt_u32"}

func TestSelectMode(t *testing.T) {
	cases := []struct {
		name     string
		wrapper  IptablesMode
		loaded   []string
		allow    bool
		want     IptablesMode
		mismatch bool
		reason   string
	}{
		{name: "nft as selected", wrapper: IptablesNFT, loaded: []string{"nf_tables"}, want: IptablesNFT},
		{name: "legacy as selected", wrapper: IptablesLegacy, loaded: allLegacy, want: IptablesLegacy},
		{name: "legacy with module loading", wrapper: IptablesLegacy, loaded: []string{"ip_tables"}, allow: true, want: IptablesLegacy},
		{
			name: "legacy falls back to nft", wrapper: IptablesLegacy, loaded: []string{"ip_tables", "nf_tables"},
			want: IptablesNFT, mismatch: true, reason: "iptable_raw, xt_tcpudp, xt_u32 are not loaded",
		},
		{
			name: "nft falls back to legacy", wrapper: IptablesNFT, loaded: allLegacy,
			want: IptablesLegacy, mismatch: true, reason: "using legacy",
		},
		{name: "nothing usable", wrapper: IptablesLegacy, loaded: []string{"ip_tables"}, want: "", reason: "xt_u32 are not loaded"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason, mismatch := selectMode(c.wrapper, loadedSet(c.loaded...), c.allow)
			if got != c.want || mismatch != c.mismatch || !strings.Contains(reason, c.reason) {
				t.Fatalf("got %q mismatch=%v reason=%q", got, mismatch, reason)
			}
		})
	}
}

func TestInstallRecoversInterrupted(t *testing.T) {
	dist, sbin := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "xtables-nft-multi"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("xtables-nft-multi", filepath.Join(dist, "iptables")); err != nil {
		t.Fatal(err)
	}
	// An earlier install died while writing the binary.
	if err := os.WriteFile(filepath.Join(sbin, "xtables-nft-multi.tmp"), []byte("b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (Iptables{Sbin: sbin, Dist: dist}).Install(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(sbin, "iptables"))
	if err != nil || string(b) != "bin" {
		t.Fatalf("iptables: %q, %v", b, err)
	}
	if _, err := os.Lstat(filepath.Join(sbin, "xtables-nft-multi.tmp")); err == nil {
		t.Fatal("temporary file left")
	}
}

func TestAvailableModules(t *testing.T) {
	dir := t.TempDir()
	sysModule := filepath.Join(dir, "module")
	if err := os.MkdirAll(filepath.Join(sysModule, "ip_tables"), 0o755); err != nil {
		t.Fatal(err)
	}
	kallsyms := filepath.Join(dir, "kallsyms")
	content := strings.Join([]string{
		"ffffffff81000000 T _stext",
		"ffffffff81a2b3c0 t u32_mt.isra.0",       // built in, with a compiler suffix
		"ffffffffc0a01000 t tcp_mt\t[xt_tcpudp]", // loaded module
		"ffffffff81a2b3c0 t iptable_raw_table_init_extra",
		"broken line",
	}, "\n")
	if err := os.WriteFile(kallsyms, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	available, err := availableModules(sysModule, kallsyms)
	if err != nil {
		t.Fatal(err)
	}
	for m, want := range map[string]bool{
		"ip_tables": true, "xt_u32": true, "xt_tcpudp": true,
		"iptable_raw": false, // only a symbol with the same prefix
		"nf_tables":   false,
	} {
		if available(m) != want {
			t.Errorf("%s: got %v, want %v", m, available(m), want)
		}
	}

	// nf_tables is taken from the wrapper decision only when it selected nft.
	if !wrapperLoaded(IptablesNFT, available)("nf_tables") || wrapperLoaded(IptablesLegacy, available)("nf_tables") {
		t.Error("nf_tables must be available only when the wrapper selected nft")
	}

	// Without kallsyms the modules found in /sys/module still count and the error is returned.
	available, err = availableModules(sysModule, filepath.Join(dir, "missing"))
	if err == nil || !available("ip_tables") || available("xt_u32") {
		t.Errorf("unreadable kallsyms: err %v, ip_tables %v, xt_u32 %v", err, available("ip_tables"), available("xt_u32"))
	}
}

func TestAvailableModulesSkipsKallsyms(t *testing.T) {
	sysModule := t.TempDir()
	for m := range moduleSymbols {
		if err := os.Mkdir(filepath.Join(sysModule, m), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Every module has a /sys/module entry: kallsyms is not opened, so a missing one is no error.
	if _, err := availableModules(sysModule, filepath.Join(sysModule, "missing")); err != nil {
		t.Fatal(err)
	}
}

func TestBackendModulesHaveSymbols(t *testing.T) {
	// A module without a symbol is never found when it is built in.
	for mode, modules := range backendModules {
		for _, m := range modules {
			if moduleSymbols[m] == "" {
				t.Errorf("%s: %s has no entry in moduleSymbols", mode, m)
			}
		}
	}
}
