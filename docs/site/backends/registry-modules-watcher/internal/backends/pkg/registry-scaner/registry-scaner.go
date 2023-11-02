package registryscaner

import (
	"context"
	"time"
	"watchdoc/internal/backends"
	"watchdoc/internal/backends/pkg/registry-scaner/cache"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type Client interface {
	Name() string
	ReleaseImage(moduleName, releaseChannel string) (v1.Image, error)
	Image(moduleName, version string) (v1.Image, error)
	ListTags(moduleName string) ([]string, error)
	Modules() ([]string, error)
}

// type registryName string
// type moduleName string
// type version string
// type releaseChannelName string
// type module struct {
// 	releaseChecksum map[releaseChannelName]string
// 	versions        map[version]docs
// }

// map[registry]map[moduleName]module
// type cache map[registryName]map[moduleName]module

// type docs struct {
// 	Tags    []string
// 	TarFile []byte
// }

type registryscaner struct {
	// registryClient   Client
	registryClients map[string]Client
	needUpdate      bool
	updateChan      chan bool
	cache           *cache.Cache
}

var releaseChannelsTags = map[string]string{
	"alpha":        "",
	"beta":         "",
	"early-access": "",
	"rock-solid":   "",
	"stable":       "",
}

// New
func New(registryClients ...Client) *registryscaner {
	registryscaner := registryscaner{
		registryClients: make(map[string]Client),
		cache:           cache.New(),
	}

	for _, client := range registryClients {
		registryscaner.registryClients[client.Name()] = client
	}

	return &registryscaner
}

func (s *registryscaner) GetState() []backends.Version {
	return s.cache.GetState()
}

// UpdateChan TODO If no one listens, the subscription is stoped.
func (s *registryscaner) UpdateChan() chan bool {
	return s.updateChan
}

// Subscribe
func (s *registryscaner) Subscribe(ctx context.Context) {
	// TODO subscribe interval from flags
	s.processRegistries(ctx)
	ticker := time.NewTicker(30 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				// s.m.Lock()

				// s.getModules(ctx)

				s.processRegistries(ctx)

				// if s.needUpdate {
				// 	s.updateChan <- s.needUpdate
				// 	s.needUpdate = false
				// }

				// s.m.Unlock()

			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

}

// func (s *subscriber) getModules(ctx context.Context) {
// 	modules, err := s.registryClient.Modules()
// 	if err != nil {
// 		klog.Fatal(err)
// 	}

// 	for _, module := range modules {
// 		tags, err := s.registryClient.ListTags(module)
// 		if err != nil {
// 			klog.Fatal(err)
// 		}

// 		versions := s.getVersions(ctx, module, tags)
// 		for version, tags := range versions {
// 			tarFile := s.getDocmentation(ctx, module, version)

// 			s.versionedDocsByModule[module][version] = docs{
// 				Tags:    tags,
// 				TarFile: tarFile,
// 			}
// 		}
// 	}
// }

// func (s *subscriber) getVersions(ctx context.Context, module string, tags []string) map[string][]string {
// 	var resp = make(map[string][]string)

// 	for _, tag := range tags {
// 		if _, ok := releaseChannelsTags[tag]; ok {
// 			releaseImage, err := s.registryClient.ReleaseImage(module, tag)
// 			if err != nil {
// 				// TODO
// 				log.Fatal(err)
// 			}

// 			// * * * * * * *
// 			releaseDigest, err := releaseImage.Digest()
// 			if err != nil {
// 				log.Fatal(err)
// 			}

// 			releaseChecksum := s.checksumsByModuleAndTag[module][tag]
// 			if releaseChecksum == releaseDigest.String() {
// 				// if checksumm is the same, skipp this pair module + tag
// 				continue
// 			}
// 			// set new checksum
// 			s.needUpdate = true
// 			s.checksumsByModuleAndTag[module][tag] = releaseDigest.String()

// 			// * * * * * * *
// 			readCloser := mutate.Extract(releaseImage)
// 			defer readCloser.Close()

// 			tarReader := tar.NewReader(readCloser)
// 			for {
// 				hdr, err := tarReader.Next()
// 				if err == io.EOF {
// 					// end of archive
// 					return resp
// 				}

// 				if err != nil {
// 					// TODO
// 					log.Fatal(err)
// 				}

// 				switch hdr.Typeflag {
// 				case tar.TypeReg:
// 					if hdr.Name == "version.json" {
// 						buf := bytes.NewBuffer(nil)
// 						_, err = io.Copy(buf, tarReader)
// 						if err != nil {
// 							// TODO
// 							log.Fatal(err)
// 						}

// 						data := make(map[string]string)           // ??? [string]interface{}
// 						err := json.Unmarshal(buf.Bytes(), &data) // TODO unmarshal to struct {version: string}
// 						if err != nil {
// 							// TODO
// 							log.Fatal(err)
// 						}

// 						if version, ok := data["version"]; ok {
// 							if tags, ok := resp[version]; ok {
// 								tags = append(tags, tag)
// 								resp[version] = tags
// 							} else {
// 								resp[version] = []string{tag}
// 							}
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return resp
// }

// func (s *subscriber) getDocmentation(ctx context.Context, module, version string) []byte {
// 	image, err := s.registryClient.Image(module, version)
// 	if err != nil {
// 		// TODO
// 		log.Fatal(err)
// 	}

// 	readCloser := mutate.Extract(image)
// 	defer readCloser.Close()

// 	var writerBuf bytes.Buffer
// 	tarWriter := tar.NewWriter(&writerBuf)

// 	tarReader := tar.NewReader(readCloser)
// 	for {
// 		hdr, err := tarReader.Next()

// 		if err == io.EOF {
// 			// end of archive
// 			if err := tarWriter.Close(); err != nil {
// 				log.Fatal(err)
// 			}

// 			return writerBuf.Bytes()
// 		}

// 		if err != nil {
// 			// TODO
// 			log.Fatal(err)
// 		}

// 		// TODO pack all from docs/*
// 		if hdr.Name == "docs/README.md" {
// 			writeTar(tarWriter, "README.md", tarReader)
// 		}

// 		if hdr.Name == "docs/README_RU.md" {
// 			writeTar(tarWriter, "README_RU.md", tarReader) // _RU or .ru
// 		}

// 		if hdr.Name == "docs/FAQ.md" {
// 			writeTar(tarWriter, "FAQ.md", tarReader)
// 		}

// 		if hdr.Name == "docs/FAQ_RU.md" {
// 			writeTar(tarWriter, "FAQ_RU.md", tarReader) // _RU or .ru
// 		}
// 	}
// }

// func writeTar(tarWriter *tar.Writer, name string, tarReader *tar.Reader) {
// 	buf := bytes.NewBuffer(nil)
// 	_, err := io.Copy(buf, tarReader)
// 	if err != nil {
// 		// TODO
// 		log.Fatal(err)
// 	}

// 	hdr := &tar.Header{
// 		Name: name,
// 		Mode: 0600,
// 		Size: int64(len(buf.Bytes())),
// 	}

// 	if err := tarWriter.WriteHeader(hdr); err != nil {
// 		log.Fatal(err)
// 	}

// 	if _, err := tarWriter.Write(buf.Bytes()); err != nil {
// 		log.Fatal(err)
// 	}
// }
