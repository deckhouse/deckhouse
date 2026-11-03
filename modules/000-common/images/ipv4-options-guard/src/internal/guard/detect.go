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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IptablesMode is the backend chosen by iptables-wrapper for the node.
type IptablesMode string

const (
	IptablesNFT    IptablesMode = "nf_tables"
	IptablesLegacy IptablesMode = "legacy"
)

// Iptables is the sbin directory managed by iptables-wrapper: the same wrapper as in the
// other Deckhouse modules decides between legacy and nft, so the guard never decides itself.
type Iptables struct {
	// Directory with the iptables symlinks; the wrapper re-points them on the first call.
	Sbin string
	// Read-only copy shipped in the image, installed into Sbin when it is empty
	// (in pods without the iptables-wrapper-init container).
	Dist string
}

// Install copies Dist into Sbin unless a working iptables is already there (populated by
// iptables-wrapper-init). Stat follows the symlink, so a dangling one is reinstalled.
// Every file is replaced atomically and iptables goes last, so an install interrupted by
// a container restart is never taken for a complete one.
func (i Iptables) Install() error {
	if _, err := os.Stat(filepath.Join(i.Sbin, "iptables")); err == nil || i.Dist == "" {
		return nil
	}
	var last string
	err := filepath.WalkDir(i.Dist, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Name() == "iptables" {
			last = path
			return nil
		}
		return installFile(path, filepath.Join(i.Sbin, d.Name()))
	})
	if err != nil || last == "" {
		return err
	}
	return installFile(last, filepath.Join(i.Sbin, "iptables"))
}

func installFile(src, dst string) error {
	tmp := dst + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, tmp); err != nil {
			return err
		}
	} else {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(tmp, b, 0o755); err != nil {
			return err
		}
	}
	return os.Rename(tmp, dst)
}

// Mode runs "iptables --version" through the wrapper (in the host netns: the CNI agent pods
// use hostNetwork) and returns the mode it selected.
func (i Iptables) Mode() (IptablesMode, error) {
	out, err := exec.Command(filepath.Join(i.Sbin, "iptables"), "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("iptables --version: %w: %s", err, strings.TrimSpace(string(out)))
	}
	switch s := string(out); {
	case strings.Contains(s, "(nf_tables)"):
		return IptablesNFT, nil
	case strings.Contains(s, "(legacy)"):
		return IptablesLegacy, nil
	default:
		return "", errors.New("unknown iptables mode: " + strings.TrimSpace(s))
	}
}

// backendModules are the kernel modules each backend needs. iptables-legacy loads the module
// of a table or a match on first use, nft loads nf_tables on the first netlink request.
var backendModules = map[IptablesMode][]string{
	// IHL is matched by native nftables expressions: through iptables-nft it would be
	// -m u32, i.e. nft_compat plus xt_u32, which is usually not loaded.
	IptablesNFT: {"nf_tables"},
	// The raw table (-t raw), the tcp match with --syn and the u32 match.
	IptablesLegacy: {"ip_tables", "iptable_raw", "xt_tcpudp", "xt_u32"},
}

// moduleSymbols are functions of each module: /proc/kallsyms lists them while the module is
// loaded or if it is built in. /sys/module misses built-in modules without visible parameters.
var moduleSymbols = map[string]string{
	"nf_tables":   "nft_do_chain",
	"ip_tables":   "ipt_do_table",
	"iptable_raw": "iptable_raw_table_init",
	"xt_tcpudp":   "tcp_mt",
	"xt_u32":      "u32_mt",
}

// availableModules reports which backend modules are loaded or built in. /proc/kallsyms is
// read only if some module has no /sys/module entry; its read error is returned for the log.
func availableModules(sysModule, kallsyms string) (func(string) bool, error) {
	found := map[string]bool{}
	want := map[string]bool{}
	for m, sym := range moduleSymbols {
		if _, err := os.Stat(filepath.Join(sysModule, m)); err == nil {
			found[m] = true
		} else {
			want[sym] = true
		}
	}
	var err error
	if len(want) > 0 {
		var f *os.File
		if f, err = os.Open(kallsyms); err == nil {
			defer f.Close()
			syms, scanErr := scanKallsyms(f, want)
			err = scanErr
			for m, sym := range moduleSymbols {
				found[m] = found[m] || syms[sym]
			}
		}
	}
	return func(m string) bool { return found[m] }, err
}

// scanKallsyms returns which of the wanted symbols are listed. Lines are
// "<addr> <type> <name>[.suffix] [module]"; compiler suffixes such as .isra.0 are cut.
func scanKallsyms(r io.Reader, want map[string]bool) (map[string]bool, error) {
	found := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		if name, _, _ := strings.Cut(fields[2], "."); want[name] {
			found[name] = true
		}
	}
	return found, sc.Err()
}

// wrapperLoaded adds nf_tables when the wrapper selected nft: it ran iptables-nft, so
// nf_tables works even if it is built in and has no /sys/module entry.
func wrapperLoaded(wrapper IptablesMode, available func(string) bool) func(string) bool {
	return func(m string) bool { return (wrapper == IptablesNFT && m == "nf_tables") || available(m) }
}

func missingModules(mode IptablesMode, loaded func(string) bool) []string {
	var missing []string
	for _, m := range backendModules[mode] {
		if !loaded(m) {
			missing = append(missing, m)
		}
	}
	return missing
}

func otherMode(mode IptablesMode) IptablesMode {
	if mode == IptablesNFT {
		return IptablesLegacy
	}
	return IptablesNFT
}

// Reason codes for the node_info metric; the full text goes to the log.
const (
	ReasonWrapperFailed    = "iptables_wrapper_failed"
	ReasonModulesNotLoaded = "kernel_modules_not_loaded"
)

// Detection is the backend chosen for the node.
type Detection struct {
	// Backend is nil if the node cannot be protected.
	Backend Backend
	// Stale is the other backend if its modules are loaded: the filter may be left there by
	// an earlier choice and is removed on the next resync.
	Stale  Backend
	Reason string
	// Code is a stable form of Reason for the metric label; empty when nothing is wrong.
	Code string
	// Mismatch: the filter uses a different mode than iptables-wrapper selected.
	Mismatch bool
}

// selectMode follows the iptables-wrapper decision. If its modules are not loaded and loading
// is not allowed, the other mode is used when all its modules are already loaded.
// Returns "" if the node cannot be protected.
func selectMode(wrapper IptablesMode, loaded func(string) bool, allowModuleLoad bool) (IptablesMode, string, bool) {
	missing := missingModules(wrapper, loaded)
	if len(missing) == 0 || allowModuleLoad {
		return wrapper, "iptables-wrapper selected " + string(wrapper), false
	}
	reason := "iptables-wrapper selected " + string(wrapper) + ", but the kernel modules " +
		strings.Join(missing, ", ") + " are not loaded and module loading is not allowed"
	other := otherMode(wrapper)
	if len(missingModules(other, loaded)) == 0 {
		return other, reason + "; using " + string(other) + ", its modules are loaded", true
	}
	return "", reason, false
}

func newBackend(mode IptablesMode, ipt Iptables) Backend {
	if mode == IptablesLegacy {
		return legacyBackend{bin: filepath.Join(ipt.Sbin, "iptables-legacy")}
	}
	return nftBackend{}
}

// DetectBackend maps the iptables-wrapper decision to a backend that needs no kernel module
// that is not already loaded, unless allowModuleLoad is set.
func DetectBackend(ipt Iptables, allowModuleLoad bool) Detection {
	wrapper, err := ipt.Mode()
	if err != nil {
		return Detection{Reason: err.Error(), Code: ReasonWrapperFailed}
	}
	if _, ok := backendModules[wrapper]; !ok {
		return Detection{Reason: "unknown iptables mode " + string(wrapper), Code: ReasonWrapperFailed}
	}
	available, kallsymsErr := availableModules("/sys/module", "/proc/kallsyms")
	loaded := wrapperLoaded(wrapper, available)
	mode, reason, mismatch := selectMode(wrapper, loaded, allowModuleLoad)
	if kallsymsErr != nil {
		// Built-in modules are only visible there: without it they are reported as not loaded.
		reason += "; built-in modules not checked: " + kallsymsErr.Error()
	}
	if mode == "" {
		return Detection{Reason: reason, Code: ReasonModulesNotLoaded}
	}
	d := Detection{Backend: newBackend(mode, ipt), Reason: reason, Mismatch: mismatch}
	if mismatch {
		d.Code = ReasonModulesNotLoaded
	}
	// Never load modules just to look for leftovers: no filter can be there without them.
	if other := otherMode(mode); len(missingModules(other, loaded)) == 0 {
		d.Stale = newBackend(other, ipt)
	}
	return d
}
