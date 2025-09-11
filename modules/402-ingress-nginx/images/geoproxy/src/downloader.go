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
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/net/context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	dbExtension = ".tar.gz"
	maxmindURL  = "https://download.maxmind.com/app/geoip_download?license_key=%v&edition_id=%v&suffix=tar.gz"
)

type Downloader struct {
	watcher     *GeoUpdaterSecret
	lastUpdated time.Time
	md5         string
	mu          sync.Mutex
	leader      *LeaderElection
}

func NewDownloader(watcher *GeoUpdaterSecret, leader *LeaderElection) *Downloader {

	return &Downloader{
		md5:     "",
		watcher: watcher,
		leader:  leader,
	}
}

func (d *Downloader) Download(_ context.Context, dstPathRoot string) error {

	if d.leader.le != nil && !d.leader.le.IsLeader() {
		log.Info(fmt.Sprintf("I'm not a leader, ignoring downloadinf GeoIP DB ..."))
		return nil
	}

	mapLicenseAndEditions := d.watcher.GetLicenseEditions()
	for licenseKey, editions := range mapLicenseAndEditions {
		for _, edition := range editions {
			url := fmt.Sprintf(maxmindURL, licenseKey, edition)
			log.Info(fmt.Sprintf("Downloading %v from Maxmind", edition))
			dataDB, err := downloadDB(url)
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

func downloadDB(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
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

func (d *Downloader) saveDB(data io.ReadCloser, dstPathRoot, edition string) (string, error) {

	absFilePath := fmt.Sprintf("%v/%v%v", dstPathRoot, edition, dbExtension)

	if err := os.MkdirAll(filepath.Dir(dstPathRoot), 0o755); err != nil {
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
		return absFilePath, nil
	}

	if err := os.Rename(tmp.Name(), absFilePath); err != nil {
		return "", fmt.Errorf("rename temp: %w", err)
	}

	d.mu.Lock()
	d.md5 = newHashSting
	d.lastUpdated = time.Now()
	d.mu.Unlock()

	return absFilePath, nil
}

func equalMD5(a, b string) bool { return strings.EqualFold(a, b) }

func getHashStringFromFile(filePath string) (string, error) {
	md5Hash := md5.New()
	file, err := os.Open(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	defer file.Close()

	if err != nil {
		return "", err
	}

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
