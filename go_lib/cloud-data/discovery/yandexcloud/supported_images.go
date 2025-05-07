package yandexcloud

import "strings"

type distroPrefix string

const (
	ubuntu = distroPrefix("ubuntu")
	debian = distroPrefix("debian")
	redos  = distroPrefix("redsoft")
)

// TODO: add more supportd
var supportedVersions = map[string]struct{}{
	// red os
	"certified-server-7-3": {},
	"certified-server-8-0": {},

	"standard-server-7-3": {},
	"standard-server-8-0": {},

	// ubuntu
	"1804": {},
	"2004": {},
	"2204": {},
	"2404": {},

	// debian
	"10": {},
	"11": {},
	"12": {},
}

func checkImageSupported(image string) bool {
	prefix, version, ok := strings.Cut(image, "-")
	if !ok {
		return false
	}

	switch prefix {
	case string(ubuntu):
		version, _, _ = strings.Cut(version, "-")
	case string(redos):
		version = strings.TrimPrefix(version, "red-os-")
	default:
		return false
	}

	_, found := supportedVersions[version]

	return found
}
