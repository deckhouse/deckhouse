/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package parser

import (
	"archive/zip"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const BduURL = "https://bdu.fstec.ru/files/documents/vulxml.zip"

type Bdu struct {
	Time    time.Time
	XMLName xml.Name   `xml:"vulnerabilities"`
	Entries []BduEntry `xml:"vul"`
}

type BduEntry struct {
	BduID   string   `xml:"identifier"`
	CveIDs  []string `xml:"identifiers>identifier"`
	Sources string   `xml:"sources"`
}

func Parse(path string) (Bdu, error) {
	var bdu Bdu
	_, err := os.Stat(path)
	if err != nil {
		return bdu, fmt.Errorf("unable to get BDU definitions (%s): %w", path, err)
	}
	defer os.RemoveAll(path)

	file, err := os.Open(path)
	if err != nil {
		return bdu, fmt.Errorf("failed to open BDU definitions: %w", err)
	}

	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return bdu, fmt.Errorf("failed to parse oval: %w", err)
	}
	xml.Unmarshal(b, &bdu)

	return bdu, nil
}

func DownloadAndExtractBdu(path string) error {
	dir, err := os.MkdirTemp(filepath.Dir(path), "db")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "vulxml.zip")

	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer out.Close()

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := http.Client{Transport: transport}

	req, err := http.NewRequest("GET", BduURL, nil)
	if err != nil {
		return err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	archive, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, f := range archive.File {
		if filepath.Base(f.Name) == filepath.Base(path) {
			zipped, err := f.Open()
			if err != nil {
				return err
			}
			defer zipped.Close()

			uncompressed, err := os.Create(path)
			if err != nil {
				return err
			}

			_, err = io.Copy(uncompressed, zipped)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}
