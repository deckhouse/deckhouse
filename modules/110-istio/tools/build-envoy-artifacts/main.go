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

// Command build-envoy-artifacts builds the two prepared bazel inputs used by the
// proxyv2 (envoy) images:
//
//	deps   external.tar.gz    -> istio/envoy-build-deps
//	       All external repositories bazel needs. Required, because the image
//	       build runs bazel with --nofetch. Fetch only, nothing is compiled.
//
//	cache  bazel-cache.tar.gz -> istio/envoy-build-cache
//	       A filled bazel --disk_cache. Only makes the build faster. Needs a
//	       full envoy compile: ~16 GB RAM and a few hours.
//
// The profiles below must match $bazelVersion, $llvmRev, $llvmCacheRev and the apt
// lists in modules/110-istio/images/proxyv2-v1xNNxN/werf.inc.yaml. Change both in
// the same commit, and rebuild the cache when the toolchain changes: bazel hashes
// the compiler, the bazel version and the build flags.
//
// Needs docker. Versions that build LLVM from source also need -source-repo
// (default $SOURCE_REPO). That is an ssh remote, so pass -ssh-key, or leave it
// unset to forward your ssh-agent.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// The container image is builder/golang-bookworm, resolved from
	// candi/base_images.yml so it always matches what CI builds with.
	// Anonymous pull works, no credentials needed.
	baseImagesFile  = "candi/base_images.yml"
	buildImageKey   = "builder/golang-bookworm"
	registryPathKey = "REGISTRY_PATH"

	// Bazel downloads with its own embedded JDK, so it needs its own trust store.
	// The system one is not used.
	cacertsImage   = "eclipse-temurin:21-jre"
	cacertsInImage = "/opt/java/openjdk/lib/security/cacerts"
	// anything smaller than this did not come out of the JDK image
	minCacertsSize = 1024

	// where -ssh-key is mounted before it is copied to /root/.ssh
	sshKeyInContainer = "/tmp/ssh-key"

	// used to check that a directory really is the deckhouse checkout
	rootMarker = "modules/110-istio/images"

	// the container script writes the suggested branch name here
	branchNameFile = "branch-name"
)

var aptClang14 = []string{
	"clang-14", "lld-14",
	"git", "git-lfs", "cmake", "ninja-build",
	"libtool", "automake", "autoconf", "make",
	"curl", "gnupg", "lsb-release",
	"python3", "python3-pip", "python3-venv",
	"libc++-14-dev", "libc++abi-14-dev",
	"libssl-dev", "libz-dev", "libunwind-14-dev", "unzip",
}

var aptLLVM17Source = []string{
	"ca-certificates",
	"git", "git-lfs", "cmake", "ninja-build", "ccache",
	"build-essential",
	"libtool", "automake", "autoconf", "make",
	"curl",
	"python3", "python3-pip", "python3-venv",
	"libc++-16-dev", "libc++abi-16-dev",
	"libssl-dev", "libz-dev", "unzip",
}

type llvmMode string

const (
	llvmAPT    llvmMode = "apt"
	llvmSource llvmMode = "source"
)

type llvmSpec struct {
	mode      llvmMode
	rev       string // source mode: llvm-project tag
	ccacheRev string // source mode: llvm-build-cache branch seeding the ccache
	prefix    string
}

type replacement struct {
	old, new string
}

type profile struct {
	bazel string
	apt   []string
	llvm  llvmSpec
	// toolchain goes into the artifact branch name. The cache is keyed on the
	// compiler, so the name has to say which one built it.
	toolchain     string
	bazelrc       []string
	workspaceSeds []replacement
	patches       []string
}

var profiles = map[string]profile{
	"1.21.6": {
		bazel:     "6.3.2",
		apt:       aptClang14,
		llvm:      llvmSpec{mode: llvmAPT, prefix: "/usr/lib/llvm-14"},
		toolchain: "bookworm",
		// the upstream 1.21.6 tag points to a broken envoy revision
		workspaceSeds: []replacement{
			{
				old: `ENVOY_SHA = "94aa5f7f82fb543e7fbc011ea398ac12cc396817"`,
				new: `ENVOY_SHA = "5c3dc559371181d5baa4a7533c36f2370fc97581"`,
			},
			{
				old: `ENVOY_SHA256 = "092784be59d19e99343afc095c1a65eca916b0b3f218d48545a6f0d43cdd5885"`,
				new: `ENVOY_SHA256 = "17f36c4267570e64123e3f08cdf8fc9442634572c5dc87f3f4192d953a4d29bf"`,
			},
		},
		patches: []string{"modules/110-istio/images/proxyv2-v1x21x6/patches/001-go-mod.patch"},
	},
	"1.25.2": {
		bazel:     "6.5.0",
		apt:       aptClang14,
		llvm:      llvmSpec{mode: llvmAPT, prefix: "/usr/lib/llvm-14"},
		toolchain: "bookworm",
	},
	"1.27.9": {
		bazel: "7.7.1",
		apt:   aptLLVM17Source,
		llvm: llvmSpec{
			mode:      llvmSource,
			rev:       "llvmorg-17.0.6",
			ccacheRev: "llvmorg-17.0.6-bookworm-gcc12-v1",
			prefix:    "/usr/lib/llvm-17",
		},
		toolchain: "bookworm-llvm17",
		bazelrc: []string{
			"build --linkopt=-L/usr/lib/llvm-16/lib",
			"build --host_linkopt=-L/usr/lib/llvm-16/lib",
		},
	},
}

func knownVersions() []string {
	versions := make([]string, 0, len(profiles))
	for v := range profiles {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return versions
}

var safeShellArg = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote quotes a string for use in the generated shell script.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if safeShellArg.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// looksLikeSSH reports true for ssh:// and for remotes like git@host:path.
func looksLikeSSH(url string) bool {
	if url == "" {
		return false
	}
	head, _, _ := strings.Cut(url, "/")
	return strings.HasPrefix(url, "ssh://") || strings.Contains(head, "@")
}

// deckhouseRoot finds the checkout with the werf files and the patches.
func deckhouseRoot(override string) (string, error) {
	if override != "" {
		root, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(root, rootMarker)); err != nil {
			return "", fmt.Errorf("%s does not look like a deckhouse checkout: no %s", root, rootMarker)
		}
		return root, nil
	}

	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", errors.New("could not determine the deckhouse root from git; pass -deckhouse-root")
	}
	root := strings.TrimSpace(string(out))
	if _, err := os.Stat(filepath.Join(root, rootMarker)); err != nil {
		return "", fmt.Errorf("git reports %s, which has no %s; pass -deckhouse-root", root, rootMarker)
	}
	return root, nil
}

// baseImage resolves an image name from candi/base_images.yml into a pullable
// reference. The file is a flat "name: digest" map with a REGISTRY_PATH key that
// prefixes every entry, so a line scan is enough and the tool needs no yaml
// dependency.
func baseImage(root, key string) (string, error) {
	path := filepath.Join(root, baseImagesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	var registry, digest string
	for _, line := range strings.Split(string(data), "\n") {
		// comments, blanks and any indented line: the map is flat, so an
		// indented key belongs to something else and must not match.
		if line == "" || strings.ContainsAny(line[:1], "# \t") {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if name != registryPathKey && name != key {
			continue
		}
		value = strings.TrimSpace(value)
		if i := strings.Index(value, " #"); i >= 0 { // drop the "# from: ..." note
			value = strings.TrimSpace(value[:i])
		}
		value = strings.Trim(value, `"`)
		if name == registryPathKey {
			registry = value
		} else {
			digest = value
		}
	}

	switch {
	case registry == "":
		return "", fmt.Errorf("%s has no %s", path, registryPathKey)
	case digest == "":
		return "", fmt.Errorf("%s has no %s", path, key)
	case !strings.HasPrefix(digest, "sha256:"):
		return "", fmt.Errorf("%s: %s is %q, want a sha256 digest", path, key, digest)
	}
	return registry + "@" + digest, nil
}

type options struct {
	version    string
	artifact   string
	out        string
	sourceRepo string
	deps       string
	sshKey     string
	bazelRoot  string
	root       string
	jobs       int
	printOnly  bool
}

// buildScript builds the shell script that runs inside the container.
func buildScript(o options, p profile, withDeps, withSSHKey bool) string {
	var s []string
	add := func(line string) {
		s = append(s, line)
	}
	addf := func(format string, args ...any) {
		s = append(s, fmt.Sprintf(format, args...))
	}

	add("set -euxo pipefail")
	add("export DEBIAN_FRONTEND=noninteractive")

	if withSSHKey {
		// Copy the key instead of using the mount: ssh refuses a key that is
		// not owned by the current user, and the mount keeps the host owner.
		add("mkdir -p /root/.ssh && chmod 700 /root/.ssh")
		addf("install -m 0600 %s /root/.ssh/id_ci", sshKeyInContainer)
		add(`printf 'StrictHostKeyChecking accept-new\n' > /root/.ssh/config`)
		add("export GIT_SSH_COMMAND='ssh -i /root/.ssh/id_ci -o IdentitiesOnly=yes " +
			"-o StrictHostKeyChecking=accept-new'")
	}

	add("apt-get update")
	addf("apt-get install -y --no-install-recommends %s", strings.Join(p.apt, " "))

	if p.llvm.mode == llvmAPT {
		addf("ln -sf %s/bin/clang   /usr/bin/clang", p.llvm.prefix)
		addf("ln -sf %s/bin/clang++ /usr/bin/clang++", p.llvm.prefix)
	} else {
		// the same LLVM the werf file builds. The shared ccache makes this take
		// minutes instead of hours.
		addf("git clone --depth 1 --branch %s %s/llvm/llvm-build-cache.git /tmp/ccache-dir",
			p.llvm.ccacheRev, o.sourceRepo)
		add("rm -rf /tmp/ccache-dir/.git")
		add("export CCACHE_DIR=/tmp/ccache-dir")
		addf("git clone --depth 1 --branch %s %s/llvm/llvm-project.git /src/llvm",
			p.llvm.rev, o.sourceRepo)
		add("mkdir -p /src/llvm/llvm/build")
		add("cd /src/llvm/llvm/build")
		add("cmake -G Ninja " +
			"-DCMAKE_BUILD_TYPE=Release " +
			`-DLLVM_ENABLE_PROJECTS="clang;lld" ` +
			`-DLLVM_TARGETS_TO_BUILD="X86" ` +
			"-DCMAKE_INSTALL_PREFIX=" + p.llvm.prefix + " " +
			"-DLLVM_INCLUDE_EXAMPLES=OFF -DLLVM_INCLUDE_TESTS=OFF " +
			"-DLLVM_INCLUDE_BENCHMARKS=OFF " +
			"-DLLVM_CCACHE_BUILD=ON -DLLVM_CCACHE_DIR=/tmp/ccache-dir " +
			`-DLLVM_CCACHE_MAXSIZE="0" ..`)
		add("ninja install")
		addf("ln -sf %s/bin/clang   /usr/bin/clang", p.llvm.prefix)
		addf("ln -sf %s/bin/clang++ /usr/bin/clang++", p.llvm.prefix)
		add("cd / && rm -rf /src/llvm /tmp/ccache-dir")
	}

	addf("curl -fsSLo /usr/local/bin/bazel "+
		"https://github.com/bazelbuild/bazel/releases/download/%s/bazel-%s-linux-x86_64",
		p.bazel, p.bazel)
	add("chmod +x /usr/local/bin/bazel")

	proxyURL := "https://github.com/istio/proxy.git"
	if o.sourceRepo != "" {
		proxyURL = o.sourceRepo + "/istio/proxy.git"
	}
	addf("git clone --depth 1 --branch %s %s /src/proxy", o.version, proxyURL)
	add("cd /src/proxy")

	for _, patch := range p.patches {
		addf("git apply --verbose /deckhouse/%s", patch)
	}

	for _, sed := range p.workspaceSeds {
		addf("sed -i %s WORKSPACE", shellQuote("s|"+sed.old+"|"+sed.new+"|"))
	}

	// The artifact branch is named after the envoy revision, because that is what
	// decides the content of both artifacts. Older versions of istio/proxy may not
	// pin it, so fall back to the istio/proxy commit.
	add(`ENVOY_SHA="$(sed -n 's/.*ENVOY_SHA *= *"\([0-9a-f]\{40\}\)".*/\1/p' WORKSPACE | head -1)"`)
	add(`if [ -z "${ENVOY_SHA}" ]; then ENVOY_SHA="$(git rev-parse HEAD)"; fi`)
	addf(`BRANCH_NAME="v%s-${ENVOY_SHA}-%s-v1"`, o.version, p.toolchain)

	// --nofetch only works when external/ is filled from a deps tarball.
	// Without it, bazel must be allowed to fetch.
	var rc []string
	if withDeps {
		rc = append(rc, "build --nofetch")
	}
	rc = append(rc, "build --disk_cache=/tmp/bazel-cache")
	rc = append(rc, p.bazelrc...)
	rc = append(rc, "build --stamp", "build --config=release")
	addf("cat >> user.bazelrc <<'EOF'\n%s\nEOF", strings.Join(rc, "\n"))

	add("if [ -f /opt/cacerts ]; then\n" +
		"  cat >> user.bazelrc <<'EOF'\n" +
		"startup --host_jvm_args=-Djavax.net.ssl.trustStore=/opt/cacerts\n" +
		"startup --host_jvm_args=-Djavax.net.ssl.trustStorePassword=changeit\n" +
		"EOF\n" +
		"fi")

	add("export GOOS=linux GOARCH=amd64 CGO_ENABLED=0")
	addf("export BAZEL_VERSION=%s USE_BAZEL_VERSION=%s", p.bazel, p.bazel)
	add("go mod download")

	if withDeps {
		// same as the werf file: fill external/ so --nofetch works
		add(`export BAZEL_OUTPUT_BASE="$(bazel info output_base)"`)
		add(`mkdir -p "${BAZEL_OUTPUT_BASE}/external"`)
		addf(`tar -zxf /deps/%s -C "${BAZEL_OUTPUT_BASE}/external"`,
			shellQuote(filepath.Base(o.deps)))
	}

	jobsFlag := ""
	if o.jobs > 0 {
		jobsFlag = fmt.Sprintf(" --jobs=%d", o.jobs)
	}
	if o.artifact == "deps" {
		// analysis only: fetches the repositories, compiles nothing
		addf("bazel build --nobuild%s //:envoy", jobsFlag)
		// local_jdk points at the JDK in this helper's build container. CI has no
		// JDK, so carrying this generated repository into the prepared deps is
		// both useless and potentially misleading.
		add(`EXTERNAL_DIR="$(bazel info output_base)/external"`)
		add(`rm -rf "${EXTERNAL_DIR}/local_jdk" "${EXTERNAL_DIR}/@local_jdk.marker"`)
		add(`if find "${EXTERNAL_DIR}" -maxdepth 1 -iname '*local_jdk*' | grep -q .; then ` +
			`echo "local_jdk entries remain after cleanup; refusing to create the archive" >&2; exit 1; fi`)
		add(`tar -czf /out/external.tar.gz -C "${EXTERNAL_DIR}" .`)
		add("ls -lh /out/external.tar.gz")
		addf(`printf '%%s\n' "${BRANCH_NAME}" > /out/%s`, branchNameFile)
	} else {
		addf("bazel build%s //:envoy", jobsFlag)
		add("tar -czf /out/bazel-cache.tar.gz -C /tmp/bazel-cache .")
		add("ls -lh /out/bazel-cache.tar.gz")
		addf(`printf '%%s\n' "${BRANCH_NAME}" > /out/%s`, branchNameFile)
	}

	return strings.Join(s, "\n") + "\n"
}

// werfDir returns the image directory for a version: 1.21.6 -> proxyv2-v1x21x6.
func werfDir(version string) string {
	return "proxyv2-v" + strings.ReplaceAll(version, ".", "x")
}

// printNextSteps says where the tarball is and what to do with it.
func printNextSteps(o options, out string) {
	tarball, repo, revVar := "external.tar.gz", "istio/envoy-build-deps", "$istioProxyDepsRev"
	if o.artifact == "cache" {
		tarball, repo, revVar = "bazel-cache.tar.gz", "istio/envoy-build-cache", "$istioProxyCacheRev"
	}

	nameFile := filepath.Join(out, branchNameFile)
	name, err := os.ReadFile(nameFile)
	if err != nil {
		return
	}
	_ = os.Remove(nameFile)

	branch := strings.TrimSpace(string(name))
	artifactPath := filepath.Join(out, tarball)
	if o.artifact == "cache" {
		fmt.Fprintf(os.Stderr, `
==> %s

    Create branch %s in %s, then extract the archive at the repository root:

        tar -xzf %s

    Commit the extracted cache directories (not bazel-cache.tar.gz itself).
    Add -v2, -v3 and so on if that branch already exists. Then set %s
    to the branch name in modules/110-istio/images/%s/werf.inc.yaml.
`, artifactPath, branch, repo, shellQuote(artifactPath), revVar, werfDir(o.version))
		return
	}

	fmt.Fprintf(os.Stderr, `
==> %s

    Push it to %s as branch:

        %s

    Add -v2, -v3 and so on if that branch already exists. Then set %s
    to the branch name in modules/110-istio/images/%s/werf.inc.yaml.
`, artifactPath, repo, branch, revVar, werfDir(o.version))
}

// extractCacerts writes a JDK trust store to path if it is not there yet. A
// leftover file from an interrupted run is re-extracted rather than trusted.
func extractCacerts(path string) error {
	if info, err := os.Stat(path); err == nil && info.Size() >= minCacertsSize {
		return nil
	}

	fmt.Fprintf(os.Stderr, "==> extracting a JDK trust store from %s\n", cacertsImage)
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()

	cmd := exec.Command("docker", "run", "--rm", cacertsImage, "cat", cacertsInImage)
	cmd.Stdout = fh
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extracting %s from %s: %w", cacertsInImage, cacertsImage, err)
	}

	info, err := fh.Stat()
	if err != nil {
		return err
	}
	if info.Size() < minCacertsSize {
		return errors.New("extracted trust store looks empty; refusing to continue")
	}
	return nil
}

func expandUser(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return filepath.Abs(path)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: go run ./build-envoy-artifacts [flags]

Builds the prepared bazel inputs used by the proxyv2 (envoy) images:

  deps   external.tar.gz -> istio/envoy-build-deps. All external repositories
         bazel needs. Fetch only, nothing is compiled.
  cache  bazel-cache.tar.gz -> istio/envoy-build-cache. A filled bazel
         --disk_cache. Needs a full envoy compile: ~16 GB RAM and a few hours.

Run it from modules/110-istio/tools. Needs docker.

Examples:
  go run ./build-envoy-artifacts -version 1.21.6 -artifact deps
  go run ./build-envoy-artifacts -version 1.27.9 -artifact deps \
      -source-repo "$SOURCE_REPO" -ssh-key ~/.ssh/id_ed25519
  go run ./build-envoy-artifacts -version 1.27.9 -artifact cache \
      -deps ./out/external.tar.gz -source-repo "$SOURCE_REPO" -jobs 8
  go run ./build-envoy-artifacts -version 1.25.2 -artifact deps -print-script

Flags:
`)
	flag.PrintDefaults()
}

func run() error {
	var o options
	flag.StringVar(&o.version, "version", "",
		"istio version to build artifacts for, one of "+strings.Join(knownVersions(), ", "))
	flag.StringVar(&o.artifact, "artifact", "",
		"deps: fetch only, cheap. cache: full envoy compile, needs ~16 GB RAM")
	flag.StringVar(&o.out, "out", "./out", "host directory for the tarball")
	flag.StringVar(&o.sourceRepo, "source-repo", os.Getenv("SOURCE_REPO"),
		"internal mirror base (default $SOURCE_REPO). Needed when LLVM is built from source.")
	flag.StringVar(&o.deps, "deps", "",
		"external.tar.gz to fill external/ before the build. Turns on --nofetch, "+
			"so the build works like CI. Use it with -artifact cache.")
	flag.StringVar(&o.sshKey, "ssh-key", "",
		"key for the internal mirrors, e.g. ~/.ssh/id_ed25519. It must have no "+
			"passphrase. Leave it unset to forward your ssh-agent instead.")
	flag.StringVar(&o.bazelRoot, "bazel-root", "./bazel-root",
		"host directory for bazel's output base. It grows to tens of GB.")
	flag.StringVar(&o.root, "deckhouse-root", "",
		"deckhouse checkout to mount, read patches from and resolve the build image "+
			"in "+baseImagesFile+" (default: git rev-parse --show-toplevel)")
	flag.IntVar(&o.jobs, "jobs", 0, "bazel --jobs, use it to limit memory on a cache build")
	flag.BoolVar(&o.printOnly, "print-script", false,
		"print the container script and exit, running nothing")
	flag.Usage = usage
	flag.Parse()

	p, ok := profiles[o.version]
	if !ok {
		return fmt.Errorf("-version: want one of %s, got %q",
			strings.Join(knownVersions(), ", "), o.version)
	}
	if o.artifact != "deps" && o.artifact != "cache" {
		return fmt.Errorf(`-artifact: want "deps" or "cache", got %q`, o.artifact)
	}
	if p.llvm.mode == llvmSource && o.sourceRepo == "" {
		return fmt.Errorf("istio %s builds LLVM from source; pass -source-repo (or set $SOURCE_REPO)",
			o.version)
	}
	if o.deps != "" && o.artifact == "deps" {
		return errors.New("-deps makes no sense with -artifact deps")
	}
	if o.deps != "" {
		if _, err := os.Stat(o.deps); err != nil {
			return fmt.Errorf("-deps: %w", err)
		}
	}
	if o.artifact == "cache" && o.deps == "" {
		fmt.Fprintln(os.Stderr, "WARNING: no -deps given, so this build may fetch from "+
			"the network, unlike CI")
	}
	if o.sshKey != "" {
		key, err := expandUser(o.sshKey)
		if err != nil {
			return fmt.Errorf("-ssh-key: %w", err)
		}
		if _, err := os.Stat(key); err != nil {
			return fmt.Errorf("-ssh-key: %w", err)
		}
		o.sshKey = key
	}

	agentSock := os.Getenv("SSH_AUTH_SOCK")
	if looksLikeSSH(o.sourceRepo) && o.sshKey == "" && agentSock == "" {
		fmt.Fprintf(os.Stderr, "WARNING: -source-repo %s looks like an ssh remote, but "+
			"there is no -ssh-key and no $SSH_AUTH_SOCK, so clones will fail\n",
			o.sourceRepo)
	}

	script := buildScript(o, p, o.deps != "", o.sshKey != "")
	if o.printOnly {
		fmt.Print(script)
		return nil
	}

	root, err := deckhouseRoot(o.root)
	if err != nil {
		return err
	}

	out, err := filepath.Abs(o.out)
	if err != nil {
		return err
	}
	bazelRoot, err := filepath.Abs(o.bazelRoot)
	if err != nil {
		return err
	}
	for _, dir := range []string{out, bazelRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	cacerts := filepath.Join(out, "cacerts")
	if err := extractCacerts(cacerts); err != nil {
		return err
	}

	args := []string{
		"run", "--rm", "-i",
		"-v", out + ":/out",
		"-v", bazelRoot + ":/root/.cache/bazel",
		"-v", cacerts + ":/opt/cacerts:ro",
		"-v", root + ":/deckhouse:ro",
	}
	if o.deps != "" {
		deps, err := filepath.Abs(o.deps)
		if err != nil {
			return err
		}
		args = append(args, "-v", filepath.Dir(deps)+":/deps:ro")
	}
	if o.sshKey != "" {
		args = append(args, "-v", o.sshKey+":"+sshKeyInContainer+":ro")
	} else if agentSock != "" {
		// no key given: forward the caller's ssh-agent. This does not work on
		// Docker Desktop, where the host socket cannot be mounted.
		args = append(args, "-v", agentSock+":"+agentSock, "-e", "SSH_AUTH_SOCK="+agentSock)
	}
	buildImage, err := baseImage(root, buildImageKey)
	if err != nil {
		return err
	}
	args = append(args, buildImage, "bash", "-s")

	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, "docker")
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	fmt.Fprintln(os.Stderr, "==> "+strings.Join(quoted, " "))

	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	printNextSteps(o, out)
	return nil
}

func main() {
	if err := run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		os.Exit(1)
	}
}
