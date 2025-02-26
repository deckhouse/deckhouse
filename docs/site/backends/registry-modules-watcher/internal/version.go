package internal

import v1 "github.com/google/go-containerregistry/pkg/v1"

type VersionData struct {
	Registry       string
	ModuleName     string
	ReleaseChannel string
	Checksum       string
	Version        string
	TarFile        []byte

	Image v1.Image
}
