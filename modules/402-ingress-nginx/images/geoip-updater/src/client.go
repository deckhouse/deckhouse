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

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxmindURL = "https://download.maxmind.com/app/geoip_download?license_key=%v&edition_id=%v&suffix=tar.gz"
)

var allowedEditions = map[string]bool{
	"GeoIP2-Anonymous-IP":    true,
	"GeoIP2-Country":         true,
	"GeoIP2-City":            true,
	"GeoIP2-Connection-Type": true,
	"GeoIP2-Domain":          true,
	"GeoIP2-ISP":             true,
	"GeoIP2-ASN":             true,
	"GeoLite2-ASN":           true,
	"GeoLite2-Country":       true,
	"GeoLite2-City":          true,
}

type Client struct {
	endpoint   string
	httpClient *http.Client
	licenseKey string
}

func NewClient(licenseKey string, opts ...Option) *Client {
	client := Client{
		endpoint:   maxmindURL,
		licenseKey: licenseKey,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(&client)
	}

	return &client
}

type Option func(*Client)

// WithEndpoint sets the base endpoint to use. By default we use
// https://download.maxmind.com/app/geoip_download?license_key=%v&edition_id=%v&suffix=tar.gz
func WithEndpoint(endpoint string) Option {
	return func(c *Client) {
		c.endpoint = endpoint
	}
}

// WithHTTPClient sets the HTTP client to use. By default we use
// http.DefaultClient.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

type DownloadResponse struct {
	// LastModified is the date that the database was last modified. It will
	// only be set if UpdateAvailable is true.
	LastModified time.Time

	// MD5 is the string representation of the new database. It will only be set
	// if UpdateAvailable is true.
	MD5 string

	// Reader can be read to access the database itself. It will only contain a
	// database if UpdateAvailable is true.
	//
	// If the Download call does not return an error, Reader will always be
	// non-nil.
	//
	// If UpdateAvailable is true, the caller must read Reader to completion and
	// close it.
	Reader io.Reader

	// UpdateAvailable is true if there is an update available for download. It
	// will be false if the MD5 used in the Download call matches what the server
	// currently has.
	UpdateAvailable bool
}

func (c *Client) Download(ctx context.Context, editionID, md5 string) (*DownloadResponse, error) {
	emptyResponse := DownloadResponse{
		Reader:          io.NopCloser(strings.NewReader("")),
		UpdateAvailable: false,
	}

	if !allowedEditions[editionID] {
		return &emptyResponse, fmt.Errorf("editionID %s is not allowed", editionID)
	}

	url := fmt.Sprintf(c.endpoint, c.licenseKey, editionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &emptyResponse, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &emptyResponse, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &emptyResponse, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	editionReader, err := makeEditionReaderFromResponse(resp)
	if err != nil {
		return &emptyResponse, fmt.Errorf("failed to create edition reader: %w", err)
	}

	newMD5 := hex.EncodeToString(editionReader.MD5.Sum(nil))

	return &DownloadResponse{
		LastModified:    time.Now(),
		MD5:             newMD5,
		Reader:          editionReader.Buffer,
		UpdateAvailable: newMD5 != md5,
	}, nil
}

func makeEditionReaderFromResponse(r *http.Response) (*editionReader, error) {
	gzReader, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, fmt.Errorf("encountered an error creating GZIP reader: %w", err)
	}
	defer func() {
		if err != nil {
			gzReader.Close()
		}
	}()

	tarReader := tar.NewReader(gzReader)

	// iterate through the tar archive to extract the mmdb file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("tar archive does not contain an mmdb file")
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar archive: %w", err)
		}

		if strings.HasSuffix(header.Name, ".mmdb") {
			break
		}
	}

	buf := new(bytes.Buffer)

	hash := md5.New()

	teeReader := io.TeeReader(tarReader, buf)

	if _, err := io.Copy(hash, teeReader); err != nil {
		return nil, err
	}

	return &editionReader{
		MD5:    hash,
		Buffer: buf,
	}, nil
}

// editionReader holds buffer and md5 hash of the edition file
type editionReader struct {
	Buffer *bytes.Buffer
	MD5    hash.Hash
}
