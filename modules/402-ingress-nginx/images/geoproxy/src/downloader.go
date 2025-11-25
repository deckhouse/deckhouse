/*
Copyright 2025 Flant JSC

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

package geodownloader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	maxmindClient "github.com/maxmind/geoipupdate/v7/client"
	"k8s.io/klog/v2"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	dbExtension    = ".tar.gz"
	maxmindURL     = "https://download.maxmind.com/app/geoip_download?license_key=%v&edition_id=%v&suffix=tar.gz"
	lockTimeLayout = time.RFC3339Nano

	headlessServiceName = "geoproxy-headless"
	kubeRBACProxyPort   = "7475" // kube-rbac-proxy serves HTTPS on 7475
)

type Downloader struct {
	watcher *GeoUpdaterSecret
	md5     map[string]string
	mu      sync.Mutex
	leader  *LeaderElection
}

func NewDownloader(watcher *GeoUpdaterSecret, leader *LeaderElection) *Downloader {
	return &Downloader{
		md5:     make(map[string]string),
		watcher: watcher,
		leader:  leader,
	}
}

func (d *Downloader) Download(ctx context.Context, dstPathRoot string, cfg *Config, force bool) error {
	mapLicenseAndEditions := d.watcher.GetLicenseEditions()
	if len(mapLicenseAndEditions) == 0 {
		klog.Infof("License editions is emty, skip downloading...")
		return nil
	}

	if !force {
		expired, err := d.LockFileIsExpired(cfg)
		if err != nil {
			return fmt.Errorf("check lock file: %w", err)
		}
		if !expired {
			return nil
		}
	}

	var errs []error
	for licenseKey, account := range mapLicenseAndEditions {
		client, clientInitialized := d.initMaxMindClient(account.AccountID, licenseKey)

		for _, edition := range account.Editions {
			if err := d.downloadEdition(ctx, dstPathRoot, licenseKey, edition, account, client, clientInitialized, cfg); err != nil {
				errs = append(errs, fmt.Errorf("license %s edition %s: %w", licenseKey, edition, err))
				log.Error(err.Error())
			}
		}
	}

	return errors.Join(errs...)
}

func (d *Downloader) initMaxMindClient(accountID int, licenseKey string) (maxmindClient.Client, bool) {
	if accountID <= 0 {
		return maxmindClient.Client{}, false
	}

	client, err := maxmindClient.New(accountID, licenseKey)
	if err != nil {
		log.Error(fmt.Sprintf("Failed init MaxMind client: %v", err))
		return maxmindClient.Client{}, false
	}

	log.Info("Client MaxMind successfully initialized!")
	return client, true
}

func (d *Downloader) downloadEdition(ctx context.Context, dstPathRoot, licenseKey, edition string, account Account, client maxmindClient.Client, clientInitialized bool, cfg *Config) error {
	currentMD5 := d.editionMD5(edition)

	var maxmindErr error
	// if not leader download db from leader
	if !d.isLeader() {
		link, err := d.waitLeaderLink(ctx, cfg.Namespace, kubeRBACProxyPort)
		if err == nil && link != "" {
			account.Mirror = link
			account.SkipTLS = true // skip TLS kubeRbacProxy
			return d.downloadFromLeader(ctx, dstPathRoot, licenseKey, edition, account)
		}
		log.Warn(fmt.Sprintf("Leader endpoint is unknown, fallback to direct download for %s: %v", edition, err))
	}

	// try download db bu official library
	if clientInitialized && account.Mirror == "" {
		handled, err := d.tryMaxMindClient(ctx, client, edition, currentMD5, dstPathRoot)
		if handled {
			return err
		}
		if err != nil {
			maxmindErr = err
			log.Error(fmt.Sprintf("Failed download GeoIP DB by MaxMind client: %v", err))
		}
	}

	// try download by manual build URL
	legacyErr := d.downloadLegacyEdition(ctx, dstPathRoot, licenseKey, edition, account)
	if legacyErr != nil && maxmindErr != nil {
		return errors.Join(maxmindErr, legacyErr)
	}

	return legacyErr
}

func (d *Downloader) tryMaxMindClient(ctx context.Context, client maxmindClient.Client, edition, currentMD5, dstPathRoot string) (bool, error) {
	downloadResp, err := client.Download(ctx, edition, currentMD5)
	if err != nil {
		incrementError(err)
		return false, fmt.Errorf("maxmind client download: %w", err)
	}

	if !downloadResp.UpdateAvailable {
		return true, nil
	}

	dbPath, err := d.saveDBFromMMDB(downloadResp.Reader, dstPathRoot, edition)
	if err != nil {
		return true, fmt.Errorf("save downloaded data: %w", err)
	}

	d.updateEditionState(edition, downloadResp.MD5)
	log.Info(fmt.Sprintf("Successfully downloaded data from %v: %v", edition, dbPath))

	return true, nil
}

func (d *Downloader) downloadLegacyEdition(ctx context.Context, dstPathRoot, licenseKey, edition string, account Account) error {
	url := createURL(account.Mirror, licenseKey, edition)
	if account.Mirror != "" {
		log.Info(fmt.Sprintf("Downloading %v from mirror: %s", edition, account.Mirror))
	} else {
		log.Info(fmt.Sprintf("Downloading %v from MaxMind", edition))
	}

	dataDB, err := downloadDB(ctx, url, account.SkipTLS)
	if err != nil {
		incrementError(err)
		return fmt.Errorf("download data: %w", err)
	}

	dbPath, err := d.saveDB(dataDB, dstPathRoot, edition)
	if err != nil {
		return fmt.Errorf("save downloading data: %w", err)
	}

	log.Info(fmt.Sprintf("Successfully downloaded data from %v: %v", edition, dbPath))
	return nil
}

func (d *Downloader) editionMD5(edition string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.md5[edition]
}

func (d *Downloader) updateEditionState(edition, newMD5 string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.md5[edition] = newMD5
}

func downloadDB(ctx context.Context, url string, skipTLSverify bool) (io.ReadCloser, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	transport.TLSClientConfig.InsecureSkipVerify = skipTLSverify

	client := &http.Client{Transport: transport, Timeout: time.Second * 3}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %v", resp.Status)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	tarArchive, err := isTarGZArchive(bytes.NewReader(buf))
	if err != nil || !tarArchive {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(buf)), nil
}

// saveDB saves a ready tar.gz stream to the destination as <edition>.tar.gz.
// Note: this function does NOT update d.md5 to avoid mixing tar.gz MD5 with MaxMind client MMDB MD5.
func (d *Downloader) saveDB(data io.ReadCloser, dstPathRoot, edition string) (string, error) {
	absFilePath := fmt.Sprintf("%v/%v%v", dstPathRoot, edition, dbExtension)

	defer data.Close()

	if err := os.MkdirAll(dstPathRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	tmp, err := os.CreateTemp(dstPathRoot, "*.mmdb")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())

	newHash := md5.New()
	if _, err := io.Copy(io.MultiWriter(tmp, newHash), data); err != nil {
		return "", fmt.Errorf("copy .mmdb: %w", err)
	}
	newHashSting := hex.EncodeToString(newHash.Sum(nil))

	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("fsync: %w", err)
	}
	defer tmp.Close()

	oldHashString, err := getHashStringFromFile(absFilePath)
	if err != nil {
		return "", fmt.Errorf("get old hash: %w", err)
	}

	if equalMD5(newHashSting, oldHashString) {
		if err := updateLockFile(); err != nil {
			return "", err
		}
		return absFilePath, nil
	}

	if err := os.Rename(tmp.Name(), absFilePath); err != nil {
		return "", fmt.Errorf("rename temp: %w", err)
	}

	if err := updateLockFile(); err != nil {
		return "", err
	}

	return absFilePath, nil
}

func (d *Downloader) downloadFromLeader(ctx context.Context, dstPathRoot, licenseKey, edition string, account Account) error {
	const maxAttempts = 3
	backoff := time.Second * 2
	var legacyErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		legacyErr = d.downloadLegacyEdition(ctx, dstPathRoot, licenseKey, edition, account)
		if legacyErr == nil {
			return nil
		}

		if attempt == maxAttempts {
			break
		}

		log.Warn(fmt.Sprintf("Failed to download %s from leader (%d/%d): %v; retry in %s", edition, attempt, maxAttempts, legacyErr, backoff))
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff *= 2 // Increase backoff timeout on x2 :2, 4 ,8
	}

	return legacyErr
}

func equalMD5(a, b string) bool { return strings.EqualFold(a, b) }

func getHashStringFromFile(filePath string) (string, error) {
	md5Hash := md5.New()
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(md5Hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(md5Hash.Sum(nil)), nil
}

func isTarGZArchive(gzipStream io.Reader) (bool, error) {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return false, fmt.Errorf("is not gzip archive: %w", err)
	}

	tarReader := tar.NewReader(uncompressedStream)
	if _, err := tarReader.Next(); err != nil {
		return false, fmt.Errorf("is not tar archive: %w", err)
	}

	return true, nil
}

func incrementError(dlErr error) {
	trimmedError := []rune(dlErr.Error())
	if len(trimmedError) > 64 {
		trimmedError = trimmedError[:64]
	}

	stringErr := string(trimmedError)

	GeoIPErrors.WithLabelValues(stringErr, "download").Inc()
}

// saveDBFromMMDB packs a raw MMDB stream into a tar.gz archive named <edition>.mmdb
// and saves it as <edition>.tar.gz in dstPathRoot.
func (d *Downloader) saveDBFromMMDB(data io.ReadCloser, dstPathRoot, edition string) (string, error) {
	absFilePath := fmt.Sprintf("%v/%v%v", dstPathRoot, edition, dbExtension)

	defer data.Close()

	if err := os.MkdirAll(dstPathRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	// Read MMDB fully to know its size for tar header
	buf, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("read mmdb: %w", err)
	}

	tmp, err := os.CreateTemp(dstPathRoot, "*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	gz := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Name: fmt.Sprintf("%s.mmdb", edition),
		Mode: 0o644,
		Size: int64(len(buf)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		tw.Close()
		gz.Close()
		tmp.Close()
		return "", fmt.Errorf("tar header: %w", err)
	}

	if _, err := tw.Write(buf); err != nil {
		tw.Close()
		gz.Close()
		tmp.Close()
		return "", fmt.Errorf("tar write: %w", err)
	}

	if err := tw.Close(); err != nil {
		gz.Close()
		tmp.Close()
		return "", fmt.Errorf("tar close: %w", err)
	}
	if err := gz.Close(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("gzip close: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close: %w", err)
	}

	// Atomically replace
	if err := os.Rename(tmp.Name(), absFilePath); err != nil {
		return "", fmt.Errorf("rename temp: %w", err)
	}

	if err := updateLockFile(); err != nil {
		return "", err
	}
	return absFilePath, nil
}

func createURL(mirror, licenseKey, dbName string) string {
	if mirror != "" {
		return fmt.Sprintf("%s/%s%s", mirror, dbName, dbExtension)
	}
	return fmt.Sprintf(maxmindURL, licenseKey, dbName)
}

func updateLockFile() error {
	ts := time.Now().UTC().Format(lockTimeLayout)
	return os.WriteFile(LastUpdateStateFile, []byte(ts), 0o644)
}

func getTimeFromLockFile() (time.Time, error) {
	rawTime, err := os.ReadFile(LastUpdateStateFile)
	if errors.Is(err, os.ErrNotExist) {
		klog.Warningf("Lock file not exists, %s", LastUpdateStateFile)
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(lockTimeLayout, strings.TrimSpace(string(rawTime)))
}

func (d *Downloader) LockFileIsExpired(cfg *Config) (bool, error) {
	lastTimeUpdate, err := getTimeFromLockFile()
	if err != nil {
		return false, err
	}

	if lastTimeUpdate.IsZero() {
		return true, nil
	}

	return time.Since(lastTimeUpdate) >= cfg.MaxmindIntervalUpdate, nil
}

// getLeaderLinkForDownload returns base URL to the leader's download endpoint.
// Returns "" if leader is unknown or we are the leader.
func (d *Downloader) getLeaderLinkForDownload(serviceName, namespace, port string) string {
	if d.isLeader() {
		return ""
	}

	if d.leader == nil || d.leader.le == nil {
		return ""
	}

	leaderPod := d.leader.le.GetLeader()
	if leaderPod == "" {
		return ""
	}

	// Use headless service to address the specific leader Pod and avoid round-robin back to ourselves.
	return fmt.Sprintf("https://%s.%s.%s.svc:%s", leaderPod, serviceName, namespace, port)
}

// waitLeaderLink waits until leader link is known or ctx expires, returning an error on timeout.
func (d *Downloader) waitLeaderLink(ctx context.Context, namespace, port string) (string, error) {
	const waitStep = time.Second
	const maxWait = 30 * time.Second

	deadline := time.Now().Add(maxWait)
	for {
		link := d.getLeaderLinkForDownload(headlessServiceName, namespace, port)
		if link != "" {
			return link, nil
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("leader endpoint is still unknown after %s", maxWait)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(waitStep):
		}
	}
}

func (d *Downloader) isLeader() bool {
	if d.leader == nil || d.leader.le == nil || !d.leader.le.IsLeader() {
		return false
	}

	return true
}
