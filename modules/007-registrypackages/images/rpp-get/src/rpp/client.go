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

package rpp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	Endpoints      []string
	Token          string
	Repository     string
	Path           string
	Retries        int
	RetryDelay     time.Duration
	Force          bool
	Extract        bool
	TempDir        string
	InstalledStore string

	RegistryDirect bool
	RegistryRepo   string
	RegistryAuth   string
	RegistryCA     string
	RegistryScheme string
}

type packageRef struct {
	raw          string
	name         string
	digest       string
	archivePath  string
	extractDir   string
	installedDir string
}

func (r packageRef) wrapErr(msg string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %s: %w", r.name, msg, err)
}

func (r packageRef) errorf(msg string, args ...any) error {
	return fmt.Errorf("%s: "+msg, append([]any{r.name}, args...)...)
}

type Client struct {
	cfg            Config
	fetcher        fetcher
	logger         *log.Logger
	resultRecorder *ResultRecorder
}

func NewClient(cfg Config, logger *log.Logger, recorder *ResultRecorder) *Client {
	return &Client{
		cfg:            cfg,
		fetcher:        newFetcher(cfg),
		logger:         logger,
		resultRecorder: recorder,
	}
}

type InstallStatus struct {
	Name      string
	raw       string
	Installed bool
}

func (c *Client) Classify(packages []string) ([]InstallStatus, error) {
	refs, err := c.newPackageRefs(packages)
	if err != nil {
		return nil, err
	}

	statuses := make([]InstallStatus, 0, len(refs))
	for _, ref := range refs {
		installed, err := c.isPackageInstalled(ref)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, InstallStatus{
			Name:      ref.name,
			raw:       ref.raw,
			Installed: installed,
		})
	}
	return statuses, nil
}

func (c *Client) UpdateAuth(endpoints []string, token string) {
	if c.cfg.RegistryDirect {
		return
	}
	c.cfg.Endpoints = endpoints
	c.cfg.Token = token
	c.fetcher = newHTTPClient(c.cfg)
}

// installWorkerCount caps install parallelism at runtime.NumCPU because install
// is CPU-bound: tar extraction + per-package install scripts (sometimes spawning
// dpkg/rpm) compete for cores, not for the network.
func installWorkerCount() int {
	workers := runtime.NumCPU()
	if workers < 1 {
		return defaultInstallWorkers
	}

	return workers
}

// fetchWorkerCount sizes parallelism for network-bound downloads. Unlike
// installWorkerCount it deliberately ignores NumCPU — fetching N small tarballs
// over an HTTP connection (often via an ssh reverse-tunnel during bootstrap)
// scales with link bandwidth, not with cores.
func fetchWorkerCount() int {
	return defaultFetchWorkers
}

func (c *Client) FetchAll(ctx context.Context, args []string) error {
	refs, err := c.newPackageRefs(args)
	if err != nil {
		return err
	}

	return c.runAll(ctx, refs, fetchWorkerCount(), c.fetchPackage)
}

func (c *Client) InstallAll(ctx context.Context, args []string) error {
	refs, err := c.newPackageRefs(args)
	if err != nil {
		return err
	}

	return c.runAll(ctx, refs, installWorkerCount(), c.installPackage)
}

func (c *Client) InstallMissing(ctx context.Context, statuses []InstallStatus) error {
	missing := make([]packageRef, 0, len(statuses))
	for _, s := range statuses {
		if s.Installed {
			if err := c.writeResult(resultSkipped, s.Name); err != nil {
				return err
			}
			continue
		}
		ref, err := c.newPackageRef(s.raw)
		if err != nil {
			return err
		}
		missing = append(missing, ref)
	}
	return c.runAll(ctx, missing, installWorkerCount(), c.installPackage)
}

func (c *Client) runAll(ctx context.Context, refs []packageRef, maxWorkers int, action func(context.Context, packageRef) error) error {
	if len(refs) == 0 {
		return nil
	}

	workerCount := min(maxWorkers, len(refs))

	if c.logger != nil {
		c.logger.Printf("processing %d packages with %d workers", len(refs), workerCount)
	}

	return runParallel(ctx, refs, workerCount, action)
}

func (c *Client) newPackageRefs(args []string) ([]packageRef, error) {
	refs := make([]packageRef, 0, len(args))
	seen := make(map[string]struct{}, len(args))

	for _, packageWithDigest := range args {
		if _, ok := seen[packageWithDigest]; ok {
			if c.logger != nil {
				c.logger.Printf("skipping duplicate package %s", packageWithDigest)
			}
			continue
		}

		ref, err := c.newPackageRef(packageWithDigest)
		if err != nil {
			return nil, err
		}

		seen[packageWithDigest] = struct{}{}
		refs = append(refs, ref)
	}

	return refs, nil
}

func (c *Client) newPackageRef(packageWithDigest string) (packageRef, error) {
	name, digest, err := parsePackageWithDigest(packageWithDigest)
	if err != nil {
		return packageRef{}, err
	}

	return packageRef{
		raw:          packageWithDigest,
		name:         name,
		digest:       digest,
		archivePath:  filepath.Join(defaultFetchedStore(c.cfg.TempDir), name, digest+".tar.gz"),
		extractDir:   filepath.Join(defaultFetchedStore(c.cfg.TempDir), name, digest+".extracted"),
		installedDir: filepath.Join(c.cfg.InstalledStore, name),
	}, nil
}

func (c *Client) installPackage(ctx context.Context, ref packageRef) error {
	return c.retry(ctx, ref, packageInstallAttempts, shouldRetryInstall, func() error {
		err := c.installPackageOnce(ctx, ref)
		if err == nil {
			return nil
		}

		c.logf(ref, "package pipeline failed: %v", err)
		c.cleanupFailedPackage(ref)
		return err
	})
}

func (c *Client) retry(ctx context.Context, ref packageRef, attempts int, shouldRetry func(error) bool, action func() error) error {
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		c.logf(ref, "attempt %d/%d", attempt, attempts)

		lastErr = action()
		if lastErr == nil {
			return nil
		}

		c.logf(ref, "attempt %d failed: %v", attempt, lastErr)

		if attempt == attempts || !shouldRetry(lastErr) {
			break
		}

		if err := waitRetry(ctx, c.cfg.RetryDelay); err != nil {
			return err
		}
	}

	return lastErr
}

func (c *Client) installPackageOnce(ctx context.Context, ref packageRef) error {
	overallStart := time.Now()
	c.logf(ref, "starting install for %s", ref.raw)

	t := time.Now()
	skip, err := c.shouldSkipInstalled(ref)
	if err != nil {
		return err
	}
	if skip {
		return c.writeResult(resultSkipped, ref.name)
	}
	skipCheckDur := time.Since(t)

	preExtracted, err := c.isExtracted(ref)
	if err != nil {
		return err
	}

	var (
		workDir    string
		fetchDur   time.Duration
		extractDur time.Duration
	)
	if preExtracted && !c.cfg.Force {
		// Prefetch (`fetch --extract`) already downloaded and decompressed this
		// package; install straight from that directory and skip the download
		// and extraction on the critical path.
		workDir = ref.extractDir
		c.logf(ref, "using pre-extracted package at %s", workDir)
	} else {
		t = time.Now()
		if err := c.ensureFetchedArchive(ctx, ref); err != nil {
			return err
		}
		fetchDur = time.Since(t)

		workDir, err = c.createWorkDir(ref)
		if err != nil {
			return err
		}

		t = time.Now()
		if err := c.extractArchive(ctx, ref, workDir); err != nil {
			return err
		}
		extractDur = time.Since(t)
	}
	defer c.cleanupWorkDir(ref, workDir)

	t = time.Now()
	if err := c.runInstallScript(ctx, ref, workDir); err != nil {
		return err
	}
	scriptDur := time.Since(t)

	t = time.Now()
	if err := c.storeInstalledPackage(ref, workDir); err != nil {
		return err
	}
	if err := c.cleanupFetchedPackage(ref); err != nil {
		return err
	}
	storeDur := time.Since(t)

	c.logf(ref, "install completed in %s (skipCheck=%s fetch=%s extract=%s script=%s store=%s)",
		time.Since(overallStart).Truncate(time.Millisecond),
		skipCheckDur.Truncate(time.Millisecond),
		fetchDur.Truncate(time.Millisecond),
		extractDur.Truncate(time.Millisecond),
		scriptDur.Truncate(time.Millisecond),
		storeDur.Truncate(time.Millisecond),
	)
	return c.writeResult(resultInstalled, ref.name)
}

func (c *Client) fetchArchive(ctx context.Context, ref packageRef) error {
	c.logf(ref, "fetching archive to %s", ref.archivePath)

	if err := os.MkdirAll(filepath.Dir(ref.archivePath), 0o755); err != nil {
		return ref.wrapErr("create fetched store", err)
	}

	if err := c.retry(ctx, ref, c.cfg.Retries, shouldRetryFetch, func() error {
		return c.downloadOnce(ctx, ref)
	}); err != nil {
		return ref.errorf("fetch %s: %w", ref.digest, err)
	}

	return nil
}

func (c *Client) downloadOnce(ctx context.Context, ref packageRef) error {
	start := time.Now()
	body, source, err := c.fetcher.Get(ctx, ref.digest)
	if err != nil {
		return err
	}
	defer body.Close()
	httpDur := time.Since(start)

	bodyStart := time.Now()
	n, err := writeResponseBody(ref.archivePath, body)
	if err != nil {
		return fmt.Errorf("write response body from %s: %w", source, err)
	}
	bodyDur := time.Since(bodyStart)

	var throughput string
	if bodyDur > 0 && n > 0 {
		mbps := float64(n) / 1024.0 / 1024.0 / bodyDur.Seconds()
		throughput = fmt.Sprintf(", %.2f MB/s", mbps)
	}
	c.logf(ref, "archive downloaded from %s: %d bytes, http=%s body=%s%s",
		source, n, httpDur.Truncate(time.Millisecond), bodyDur.Truncate(time.Millisecond), throughput)
	return nil
}

func (c *Client) fetchPackage(ctx context.Context, ref packageRef) error {
	skip, err := c.shouldSkipInstalled(ref)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	if c.cfg.Extract {
		return c.ensureExtracted(ctx, ref)
	}

	return c.ensureFetchedArchive(ctx, ref)
}

func (c *Client) ensureExtracted(ctx context.Context, ref packageRef) error {
	if !c.cfg.Force {
		extracted, err := c.isExtracted(ref)
		if err != nil {
			return err
		}
		if extracted {
			c.logf(ref, "'%s' package already extracted", ref.raw)
			return nil
		}
	}

	c.logf(ref, "downloading and extracting '%s' to %s", ref.raw, ref.extractDir)
	if err := c.retry(ctx, ref, c.cfg.Retries, shouldRetryFetch, func() error {
		return c.downloadAndExtractOnce(ctx, ref)
	}); err != nil {
		return ref.errorf("download+extract %s: %w", ref.digest, err)
	}
	return nil
}

func (c *Client) isExtracted(ref packageRef) (bool, error) {
	entries, err := os.ReadDir(ref.extractDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, ref.wrapErr("read extract dir", err)
	}
	return len(entries) > 0, nil
}

func (c *Client) downloadAndExtractOnce(ctx context.Context, ref packageRef) error {
	if err := os.RemoveAll(ref.extractDir); err != nil {
		return ref.wrapErr("clean extract dir", err)
	}
	if err := os.MkdirAll(ref.extractDir, 0o755); err != nil {
		return ref.wrapErr("create extract dir", err)
	}

	start := time.Now()
	body, source, err := c.fetcher.Get(ctx, ref.digest)
	if err != nil {
		return err
	}
	defer body.Close()

	if err := extractTarGzStream(ctx, body, ref.extractDir); err != nil {
		return ref.errorf("stream extract %s: %w", ref.digest, err)
	}

	c.logf(ref, "downloaded+extracted from %s in %s", source, time.Since(start).Truncate(time.Millisecond))
	return nil
}

func (c *Client) shouldSkipInstalled(ref packageRef) (bool, error) {
	if c.cfg.Force {
		c.logf(ref, "force mode enabled, skipping installed-package check")
		return false, nil
	}

	installed, err := c.isPackageInstalled(ref)
	if err != nil {
		return false, err
	}
	if installed {
		c.logf(ref, "'%s' package already installed", ref.raw)
		return true, nil
	}

	return false, nil
}

func (c *Client) ensureFetchedArchive(ctx context.Context, ref packageRef) error {
	if c.cfg.Force {
		c.logf(ref, "force mode enabled, downloading archive again")
		return c.fetchArchive(ctx, ref)
	}

	fetched, err := c.isPackageFetched(ref)
	if err != nil {
		return err
	}
	if fetched {
		c.logf(ref, "'%s' package already fetched", ref.raw)
		return nil
	}

	c.logf(ref, "'%s' package not found locally", ref.raw)
	return c.fetchArchive(ctx, ref)
}

func (c *Client) UninstallAll(ctx context.Context, packages []string) error {
	refs := make([]packageRef, 0, len(packages))
	for _, packageName := range packages {
		refs = append(refs, packageRef{
			raw:          packageName,
			name:         packageName,
			installedDir: filepath.Join(c.cfg.InstalledStore, packageName),
		})
	}

	return runParallel(ctx, refs, 1, c.uninstallPackageRef)
}

func (c *Client) uninstallPackageRef(ctx context.Context, ref packageRef) error {
	scriptPath := filepath.Join(ref.installedDir, "uninstall")
	info, exists, err := statPath(scriptPath)
	if err != nil {
		return ref.wrapErr("stat uninstall script", err)
	}
	if !exists {
		c.logf(ref, "package is not installed, skipping uninstall")
		return c.writeResult(resultSkipped, ref.name)
	}
	if info.IsDir() {
		return ref.errorf("uninstall script path is a directory")
	}

	c.logf(ref, "removing package")
	if err := c.runCommand(ctx, ref.installedDir, "bash", scriptPath); err != nil {
		return ref.wrapErr("run uninstall script", err)
	}

	if err := os.RemoveAll(ref.installedDir); err != nil {
		return ref.wrapErr("cleanup installed package", err)
	}

	c.logf(ref, "package removed")
	return c.writeResult(resultRemoved, ref.name)
}

func (c *Client) isPackageInstalled(ref packageRef) (bool, error) {
	digestPath := filepath.Join(ref.installedDir, "digest")
	content, err := os.ReadFile(digestPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, ref.wrapErr("read installed digest", err)
	}

	return strings.TrimSpace(string(content)) == ref.digest, nil
}

func (c *Client) isPackageFetched(ref packageRef) (bool, error) {
	info, exists, err := statPath(ref.archivePath)
	if err != nil {
		return false, ref.wrapErr("stat fetched archive", err)
	}
	if !exists {
		return false, nil
	}

	return !info.IsDir() && info.Size() > 0, nil
}

func (c *Client) createWorkDir(ref packageRef) (string, error) {
	c.logf(ref, "creating temporary workdir in %s", c.cfg.TempDir)

	workDir, err := os.MkdirTemp(c.cfg.TempDir, "rpp-get-")
	if err != nil {
		return "", ref.wrapErr("create temp workdir", err)
	}

	c.logf(ref, "temporary workdir is %s", workDir)
	return workDir, nil
}

func (c *Client) cleanupWorkDir(ref packageRef, workDir string) {
	c.logf(ref, "removing temporary workdir %s", workDir)
	if err := os.RemoveAll(workDir); err != nil {
		c.logf(ref, "failed to remove temporary workdir: %v", err)
	}
}

func (c *Client) extractArchive(ctx context.Context, ref packageRef, workDir string) error {
	c.logf(ref, "extracting archive into %s", workDir)
	if err := extractTarGz(ctx, ref.archivePath, workDir); err != nil {
		return ref.errorf("extract %s: %w", ref.archivePath, err)
	}

	c.logf(ref, "archive extracted")
	return nil
}

func (c *Client) runInstallScript(ctx context.Context, ref packageRef, workDir string) error {
	c.logf(ref, "running install script")
	if err := c.runCommand(ctx, workDir, "./install"); err != nil {
		return ref.wrapErr("run install script", err)
	}

	c.logf(ref, "install script finished successfully")
	return nil
}

func (c *Client) runCommand(ctx context.Context, dir, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, scriptExecTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (c *Client) storeInstalledPackage(ref packageRef, workDir string) error {
	c.logf(ref, "storing installed package metadata in %s", ref.installedDir)

	if err := os.MkdirAll(ref.installedDir, 0o755); err != nil {
		return ref.wrapErr("create installed store", err)
	}

	if err := writeDigestFile(ref); err != nil {
		return err
	}

	if err := copyPackageScripts(ref, workDir); err != nil {
		return err
	}

	c.logf(ref, "installed package metadata stored")
	return nil
}

func writeDigestFile(ref packageRef) error {
	digestPath := filepath.Join(ref.installedDir, "digest")
	if err := os.WriteFile(digestPath, []byte(ref.digest+"\n"), 0o644); err != nil {
		return ref.wrapErr("write digest", err)
	}

	return nil
}

func copyPackageScripts(ref packageRef, workDir string) error {
	for _, name := range packageScripts {
		src := filepath.Join(workDir, name)
		dst := filepath.Join(ref.installedDir, name)

		if err := copyFile(src, dst); err != nil {
			return ref.errorf("copy %s: %w", name, err)
		}
	}

	return nil
}

func (c *Client) cleanupFetchedPackage(ref packageRef) error {
	cacheDir := filepath.Join(defaultFetchedStore(c.cfg.TempDir), ref.name)
	c.logf(ref, "removing fetched cache %s", cacheDir)

	if err := os.RemoveAll(cacheDir); err != nil {
		return ref.wrapErr("cleanup fetched package", err)
	}

	return nil
}

func (c *Client) cleanupFailedPackage(ref packageRef) {
	if err := os.RemoveAll(ref.installedDir); err != nil {
		c.logf(ref, "failed to remove installed package dir: %v", err)
	}

	if err := c.cleanupFetchedPackage(ref); err != nil {
		c.logf(ref, "failed to remove fetched cache: %v", err)
	}
}

func (c *Client) logf(ref packageRef, format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf("[%s] %s", ref.name, fmt.Sprintf(format, args...))
	}
}

func (c *Client) writeResult(action, packageName string) error {
	if err := c.resultRecorder.record(action, packageName); err != nil {
		return fmt.Errorf("%s: record %s result: %w", packageName, action, err)
	}

	return nil
}

func shouldRetryInstall(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
