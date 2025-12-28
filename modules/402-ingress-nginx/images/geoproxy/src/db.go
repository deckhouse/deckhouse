package geodownloader

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
)

type GeoDB struct {
	MU      sync.Mutex
	readers map[string]*geoip2.Reader
}

func NewGeoDB(mmdbDirPath string) (*GeoDB, error) {
	var mmdbFilesPath []string

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

	geoDB := &GeoDB{
		readers: make(map[string]*geoip2.Reader, len(mmdbFilesPath)),
	}

	sort.Strings(mmdbFilesPath)

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

func (g *GeoDB) Close() {
	g.MU.Lock()
	defer g.MU.Unlock()
	for _, r := range g.readers {
		r.Close()
	}
}
