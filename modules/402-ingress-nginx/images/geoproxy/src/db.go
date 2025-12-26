package geodownloader

import (
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
	mu      sync.Mutex
	readers map[string]*geoip2.Reader
}
