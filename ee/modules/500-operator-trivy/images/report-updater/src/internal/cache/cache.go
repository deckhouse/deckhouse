/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	dockerConfigPath      = "/root/.docker/config.json"
	bduDictionaryFilename = "export.json"
	tarGzMediaType        = "application/deckhouse.io.bdu.layer.v1.tar+gzip"
)

type Cache interface {
	Get(string) ([]string, bool)
	Check() error
	Renew(ctx context.Context) error
}

type VulnerabilityCache struct {
	logger *log.Logger

	dict         VulnerabilityDictionary
	mu           sync.RWMutex
	sourceConfig RegistryConfig
}

type VulnerabilityDictionary struct {
	TS   time.Time           `json:"timestamp"`
	Data map[string][]string `json:"data"`
}

type RegistryConfig struct {
	registry   string
	repository string
	tag        string
	user       string
	password   string
}

type ContainerRegistry struct {
	Auth string `json:"auth"`
}

type DockerConfig struct {
	Auths map[string]ContainerRegistry `json:"auths"`
}

func NewVulnerabilityCache(ctx context.Context, logger *log.Logger) (*VulnerabilityCache, error) {
	image := os.Getenv("DICTIONARY_OCI_IMAGE")
	if len(image) == 0 {
		return nil, fmt.Errorf("DICTIONARY_OCI_IMAGE env not set")
	}

	ref, err := name.ParseReference(image, name.StrictValidation)
	if err != nil {
		return nil, fmt.Errorf("parse the '%s' image: %w", image, err)
	}

	registry := ref.Context().RegistryStr()

	dockerConfigFile, err := os.Open(dockerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("open docker config.json: %w", err)
	}
	defer dockerConfigFile.Close()

	dockerConfig := new(DockerConfig)
	if err = json.NewDecoder(dockerConfigFile).Decode(dockerConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal docker config.json: %w", err)
	}

	var (
		user     string
		password string
	)

	containerRegistry, ok := dockerConfig.Auths[registry]
	if !ok {
		logger.Printf("failed to find auth config for bdu registry %s in docker file\n", registry)
	} else {
		if len(containerRegistry.Auth) == 0 {
			return nil, fmt.Errorf("bdu registry auth config is empty")
		}

		decoded, err := base64.StdEncoding.DecodeString(containerRegistry.Auth)
		if err != nil {
			return nil, fmt.Errorf("decode bdu registry auth config: %w", err)
		}

		auth := string(decoded)
		if len(strings.Split(auth, ":")) < 2 {
			return nil, fmt.Errorf("bdu registry auth config seems to be malformed, should have the following format: 'user:password'")
		}

		user = strings.Split(auth, ":")[0]
		password = strings.Split(auth, ":")[1]
	}

	cache := &VulnerabilityCache{
		logger: logger,
		dict: VulnerabilityDictionary{
			Data: make(map[string][]string),
		},
		sourceConfig: RegistryConfig{
			registry:   registry,
			repository: ref.Context().RepositoryStr(),
			tag:        ref.Identifier(),
			user:       user,
			password:   password,
		},
	}

	if err = cache.initDictionary(ctx); err != nil {
		return nil, err
	}

	return cache, nil
}

func (c *VulnerabilityCache) initDictionary(ctx context.Context) error {
	c.logger.Println("initialize BDU dictionary")
	if err := c.Renew(ctx); err != nil {
		c.logger.Println("failed to initialize BDU dictionary")
		return err
	}

	return nil
}

func (c *VulnerabilityCache) Renew(ctx context.Context) error {
	if err := c.getData(ctx); err != nil {
		return fmt.Errorf("renew BDU base: get data from image descriptors: %w", err)
	}

	return nil
}

func (c *VulnerabilityCache) getData(ctx context.Context) error {
	c.logger.Println("download BDU image")

	// create oras in-memory storage
	store := memory.New()

	// set target repository
	repo, err := remote.NewRepository(c.sourceConfig.registry + "/" + c.sourceConfig.repository)
	if err != nil {
		return err
	}

	// customize http client transport
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if os.Getenv("INSECURE_REGISTRY") == "true" {
		repo.PlainHTTP = true
	} else {
		// add tls config
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
		}
		registryCA := os.Getenv("CUSTOM_REGISTRY_CA")
		// add custom ca
		if len(registryCA) != 0 {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				return fmt.Errorf("get system cert pool: %w", err)
			}

			if !certPool.AppendCertsFromPEM([]byte(registryCA)) {
				c.logger.Println("parse registry CA error")
			}
			tlsConfig.RootCAs = certPool
		}
		transport.TLSClientConfig = tlsConfig
	}

	// set repository auth
	repo.Client = &auth.Client{
		Client: &http.Client{
			Transport: transport,
		},
		Cache: auth.DefaultCache,
		Credential: auth.StaticCredential(c.sourceConfig.registry, auth.Credential{
			Username: c.sourceConfig.user,
			Password: c.sourceConfig.password,
		}),
	}

	// copy the requested image from remote repository to oras in-memory storage and save its descriptor
	descriptor, err := oras.Copy(ctx, repo, c.sourceConfig.tag, store, c.sourceConfig.tag, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("ren BDU base: copy BDU image to memory: %w", err)
	}

	// get successor descriptors of the descriptor
	successors, err := content.Successors(ctx, store, descriptor)
	if err != nil {
		return fmt.Errorf("renew BDU base: get descriptors from BDU image: %w", err)
	}

	// iterate over descriptors to get the ones with relevant MediaType
	for _, desc := range successors {
		switch desc.MediaType {
		case tarGzMediaType:
			if err = c.processDescriptor(ctx, store, desc); err != nil {
				return fmt.Errorf("renew BDU base: process tar archive: %w", err)
			}
		default:
			//skip
		}
	}

	return nil
}

func (c *VulnerabilityCache) processDescriptor(ctx context.Context, store *memory.Store, desc ocispec.Descriptor) error {
	tarGz, err := store.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("renew BDU base: fetch tar archive: %w", err)
	}
	defer tarGz.Close()

	gzipReader, err := gzip.NewReader(tarGz)
	if err != nil {
		return fmt.Errorf("renew BDU base: uncompress tar archive: %w", err)
	}

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("renew BDU base: iterate over tar: %w", err)
		}

		if header.Name == bduDictionaryFilename {
			tempDict := &VulnerabilityDictionary{
				Data: make(map[string][]string),
			}

			if err = json.NewDecoder(tarReader).Decode(tempDict); err != nil {
				return fmt.Errorf("renew BDU base: unmarshal BDU dictionary: %w", err)
			}

			if len(tempDict.Data) == 0 {
				return fmt.Errorf("renew BDU base: dictionary is empty")
			}

			if tempDict.TS != c.dict.TS {
				c.mu.Lock()
				c.dict.Data = tempDict.Data
				c.dict.TS = tempDict.TS
				c.mu.Unlock()
				c.logger.Printf("BDU dictionary dated %v has been applied", c.dict.TS)
			} else {
				c.logger.Printf("BDU dictionary is up to date (ts: %s)", c.dict.TS)
			}

			break
		}
	}

	return nil
}

func (c *VulnerabilityCache) Get(vuln string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.dict.Data[vuln]
	return entry, ok
}

// TODO: think about healthz check
func (c *VulnerabilityCache) Check() error {
	if len(c.dict.Data) == 0 {
		return fmt.Errorf("BDU dictionary empty")
	}
	return nil
}
