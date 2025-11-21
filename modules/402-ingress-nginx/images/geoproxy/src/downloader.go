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

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	dbExtension = ".tar.gz"
	maxmindURL  = "https://download.maxmind.com/app/geoip_download?license_key=%v&edition_id=%v&suffix=tar.gz"
)

type Downloader struct {
	watcher     *GeoUpdaterSecret
	lastUpdated time.Time
	md5         map[string]string
	mu          sync.Mutex
	leader      *LeaderElection
}

func NewDownloader(watcher *GeoUpdaterSecret, leader *LeaderElection) *Downloader {
	return &Downloader{
		md5:     make(map[string]string),
		watcher: watcher,
		leader:  leader,
	}
}

func (d *Downloader) Download(ctx context.Context, dstPathRoot string) error {
	// If leader elector is not ready or this instance is not leader, skip work.
	if d.leader == nil || d.leader.le == nil || !d.leader.le.IsLeader() {
		log.Info("I'm not a leader, ignoring downloading GeoIP DB ...")
		return nil
	}

	mapLicenseAndEditions := d.watcher.GetLicenseEditions()
	for licenseKey, account := range mapLicenseAndEditions {
		accountID := account.AccountID
		clientInitialized := false

		var (
			client maxmindClient.Client
			err    error
		)

		if accountID > 0 {
			client, err = maxmindClient.New(accountID, licenseKey)
			if err != nil {
				// Fallback to legacy later.
				log.Error(fmt.Sprintf("Failed init MaxMind client: %v", err))
			} else {
				clientInitialized = true
				log.Info("Client MaxMind successfully initialized!")
			}
		}

		for _, edition := range account.Editions {
			// Try download via official MaxMind client if available
			d.mu.Lock()
			md5 := d.md5[edition]
			d.mu.Unlock()

			if clientInitialized && account.Mirror == "" {
				downloadResp, err := client.Download(ctx, edition, md5)
				if err != nil {
					// Record the error and fallback to legacy method below.
					incrementError(err)
					log.Error(fmt.Sprintf("Failed download GeoIP DB by MaxMind client: %v", err))
				} else {
					if downloadResp.UpdateAvailable {
						dbPath, err := d.saveDBFromMMDB(downloadResp.Reader, dstPathRoot, edition)
						if err != nil {
							log.Error(fmt.Sprintf("Error save downloading data from %v: %v", edition, err))
						} else {
							d.mu.Lock()
							d.md5[edition] = downloadResp.MD5
							d.lastUpdated = time.Now()
							d.mu.Unlock()
							log.Info(fmt.Sprintf("Successfully downloaded data from %v: %v", edition, dbPath))
						}
						// No need to try legacy if client already handled this edition
						continue
					}
					// No update available via client – skip legacy download to avoid redundant work.
					continue
				}
			}

			// Try download as legacy option
			url := createURL(account.Mirror, licenseKey, edition)
			if account.Mirror != "" {
				log.Info(fmt.Sprintf("Downloading %v from mirror: %s", edition, account.Mirror))
			} else {
				log.Info(fmt.Sprintf("Downloading %v from MaxMind", edition))
			}
			dataDB, err := downloadDB(url, account.SkipTLS)
			if err != nil {
				incrementError(err)
				log.Error(fmt.Sprintf("Error downloading data from %v: %v", edition, err))
				continue
			}

			dbPath, err := d.saveDB(dataDB, dstPathRoot, edition)
			if err != nil {
				log.Error(fmt.Sprintf("Error save downloading data from %v: %v", edition, err))
				continue
			}

			log.Info(fmt.Sprintf("Successfully downloaded data from %v: %v", edition, dbPath))
		}
	}

	return nil
}

func downloadDB(url string, skipTLSverify bool) (io.ReadCloser, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

transport.TLSClientConfig.InsecureSkipVerify = skipTLSverify

	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
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

	if err := os.MkdirAll(dstPathRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	defer data.Close()

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
		return absFilePath, nil
	}

	if err := os.Rename(tmp.Name(), absFilePath); err != nil {
		return "", fmt.Errorf("rename temp: %w", err)
	}

	d.mu.Lock()
	d.lastUpdated = time.Now()
	d.mu.Unlock()

	return absFilePath, nil
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

	if err := os.MkdirAll(dstPathRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	defer data.Close()

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

	d.mu.Lock()
	d.lastUpdated = time.Now()
	d.mu.Unlock()

	return absFilePath, nil
}

func createURL(mirror, licenseKey, dbName string) string {
	if mirror != "" {
		return fmt.Sprintf("%s/%s%s", mirror, dbName, dbExtension)
	}
	return fmt.Sprintf(maxmindURL, licenseKey, dbName)
}
