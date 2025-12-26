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
	"bytes"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/coocood/freecache"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/oschwald/geoip2-golang"
)

// List GeoIP DB from CRD
const (
	GeoIP2AnonymousIP    string = "GeoIP2-Anonymous-IP"
	GeoIP2Country        string = "GeoIP2-Country"
	GeoIP2City           string = "GeoIP2-City"
	GeoIP2ConnectionType string = "GeoIP2-Connection-Type"
	GeoIP2Domain         string = "GeoIP2-Domain"
	GeoIP2ISP            string = "GeoIP2-ISP"
	GeoIP2ASN            string = "GeoIP2-ASN"
	GeoLite2ASN          string = "GeoLite2-ASN"
	GeoLite2Country      string = "GeoLite2-Country"
	GeoLite2City         string = "GeoLite2-City"

	// HEADERS
	geoip2CountryCode string = "x-geoip-country-code"
	geoip2City        string = "x-geoip-city"
	geoip2RegionName  string = "x-geoip-region-name"
	geoip2Latitude    string = "x-geoip-latitude"
	geoip2Longitude   string = "x-geoip-longitude"
)

const (
	geoCacheKeyPrefix  = "geoip:"
	geoCacheTTLSeconds = 3600

	// freecache.NewCache has a minimum internal size of 512KiB; keep it larger to reduce churn.
	geoCacheSizeBytes = 5 * 512 * 1024
)

type GeoDB struct {
	MU         sync.RWMutex
	readers    map[string]*geoip2.Reader
	readerKeys []string
	cache      *freecache.Cache
}

func NewGeoDB(mmdbDirPath string) (*GeoDB, error) {
	var mmdbFilesPath []string

	if err := os.MkdirAll(mmdbDirPath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %q: %w", mmdbDirPath, err)
	}

	if err := filepath.WalkDir(mmdbDirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".mmdb") {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			mmdbFilesPath = append(mmdbFilesPath, absPath)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	sort.Strings(mmdbFilesPath)

	geoDB := &GeoDB{
		readers:    make(map[string]*geoip2.Reader, len(mmdbFilesPath)),
		readerKeys: append([]string(nil), mmdbFilesPath...),
		cache:      freecache.NewCache(geoCacheSizeBytes),
	}

	for i := range mmdbFilesPath {
		fileGeoDB := mmdbFilesPath[i]
		reader, err := geoip2.Open(fileGeoDB)
		if err != nil {
			geoDB.Close()

			return nil, err
		}

		geoDB.readers[fileGeoDB] = reader
	}

	return geoDB, nil
}

func (g *GeoDB) ClearCache() {
	if g.cache != nil {
		g.cache.Clear()
	}
}

func (g *GeoDB) Reload(mmdbDirPath string) error {
	var mmdbFilesPath []string

	if err := os.MkdirAll(mmdbDirPath, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", mmdbDirPath, err)
	}

	if err := filepath.WalkDir(mmdbDirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".mmdb") {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			mmdbFilesPath = append(mmdbFilesPath, absPath)
		}

		return nil
	}); err != nil {
		return err
	}

	if len(mmdbFilesPath) == 0 {
		return fmt.Errorf("no mmdb files found in %q", mmdbDirPath)
	}

	sort.Strings(mmdbFilesPath)

	newReaders := make(map[string]*geoip2.Reader, len(mmdbFilesPath))
	for i := range mmdbFilesPath {
		fileGeoDB := mmdbFilesPath[i]
		reader, err := geoip2.Open(fileGeoDB)
		if err != nil {
			for _, r := range newReaders {
				r.Close()
			}
			return err
		}

		newReaders[fileGeoDB] = reader
	}

	g.MU.Lock()
	oldReaders := g.readers
	g.readers = newReaders
	g.readerKeys = append([]string(nil), mmdbFilesPath...)
	g.MU.Unlock()

	for _, r := range oldReaders {
		r.Close()
	}

	g.ClearCache()

	return nil
}

func (g *GeoDB) GetGeoHeaders(ip string) ([]*corev3.HeaderValueOption, bool, error) {
	cacheKey := []byte(geoCacheKeyPrefix + ip)
	if g.cache != nil {
		if payload, err := g.cache.Get(cacheKey); err == nil {
			countryCode, city, ok := decodeGeoCache(payload)
			if ok {
				return buildGeoHeaders(countryCode, city), true, nil
			}
		}
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, false, fmt.Errorf("bad ip: %q", ip)
	}

	countryCode, city, err := g.lookupGeoValues(parsedIP)
	if err != nil {
		return nil, false, err
	}

	if g.cache != nil {
		_ = g.cache.Set(cacheKey, encodeGeoCache(countryCode, city), geoCacheTTLSeconds)
	}

	return buildGeoHeaders(countryCode, city), false, nil
}

func buildGeoHeaders(countryCode, city string) []*corev3.HeaderValueOption {
	setHeaders := make([]*corev3.HeaderValueOption, 0, 2)
	if countryCode != "" {
		setHeaders = append(setHeaders, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{
				Key:      geoip2CountryCode,
				Value:    countryCode,
				RawValue: []byte(countryCode),
			},
			AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
		})
	}
	if city != "" {
		setHeaders = append(setHeaders, &corev3.HeaderValueOption{
			Header: &corev3.HeaderValue{
				Key:      geoip2City,
				Value:    city,
				RawValue: []byte(city),
			},
			AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
		})
	}
	return setHeaders
}

func encodeGeoCache(countryCode, city string) []byte {
	if countryCode == "" && city == "" {
		return []byte{0}
	}
	out := make([]byte, 0, len(countryCode)+1+len(city))
	out = append(out, countryCode...)
	out = append(out, 0)
	out = append(out, city...)
	return out
}

func decodeGeoCache(payload []byte) (countryCode, city string, ok bool) {
	if len(payload) == 1 && payload[0] == 0 {
		return "", "", true
	}
	sep := bytes.IndexByte(payload, 0)
	if sep < 0 {
		return "", "", false
	}
	return string(payload[:sep]), string(payload[sep+1:]), true
}

func (g *GeoDB) lookupGeoValues(parsedIP net.IP) (countryCode, city string, err error) {
	g.MU.RLock()
	defer g.MU.RUnlock()

	haveCountry := false
	haveCity := false

	for _, k := range g.readerKeys {
		db := g.readers[k]

		cityRec, err := db.City(parsedIP)
		if err == nil && cityRec != nil {
			if name := cityRec.City.Names["en"]; name != "" {
				city = name
				haveCity = true
			}
		}

		countryRec, err := db.Country(parsedIP)
		if err == nil && countryRec != nil {
			if isoCode := countryRec.Country.IsoCode; isoCode != "" {
				countryCode = isoCode
				haveCountry = true
			}
		}

		if haveCountry && haveCity {
			break
		}
	}

	return countryCode, city, nil
}

func (g *GeoDB) Close() {
	g.MU.Lock()
	defer g.MU.Unlock()
	for _, r := range g.readers {
		r.Close()
	}
}
