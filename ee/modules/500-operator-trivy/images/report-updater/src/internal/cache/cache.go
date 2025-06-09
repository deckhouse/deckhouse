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

type VulnerabilityCache struct {
	logger *log.Logger
	dict   Dictionary
	mtx    sync.RWMutex
	config RegistryConfig
}

type Dictionary struct {
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

func New(ctx context.Context, logger *log.Logger) (*VulnerabilityCache, error) {
	image := os.Getenv("DICTIONARY_OCI_IMAGE")
	if len(image) == 0 {
		return nil, fmt.Errorf("DICTIONARY_OCI_IMAGE env not set")
	}

	ref, err := name.ParseReference(image, name.StrictValidation)
	if err != nil {
		return nil, fmt.Errorf("parse the '%s' image: %w", image, err)
	}

	dockerConfigFile, err := os.Open(dockerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("open docker config.json: %w", err)
	}
	defer dockerConfigFile.Close()

	dockerConfig := new(DockerConfig)
	if err = json.NewDecoder(dockerConfigFile).Decode(dockerConfig); err != nil {
		return nil, fmt.Errorf("unmarshal docker config.json: %w", err)
	}

	registry := ref.Context().RegistryStr()
	user, password, err := parseAuthConfig(dockerConfig, registry, logger)
	if err != nil {
		return nil, fmt.Errorf("parse docker config.json: %w", err)
	}

	cache := &VulnerabilityCache{
		logger: logger,
		dict: Dictionary{
			Data: make(map[string][]string),
		},
		config: RegistryConfig{
			registry:   registry,
			repository: ref.Context().RepositoryStr(),
			tag:        ref.Identifier(),
			user:       user,
			password:   password,
		},
	}

	if err = cache.initDictionary(ctx); err != nil {
		return nil, fmt.Errorf("init dictionary: %w", err)
	}

	return cache, nil
}

func parseAuthConfig(config *DockerConfig, registry string, logger *log.Logger) (string, string, error) {
	containerRegistry, ok := config.Auths[registry]
	if !ok {
		logger.Printf("failed to find auth config for the '%s' registry in docker file\n", registry)
		return "", "", nil
	}

	if len(containerRegistry.Auth) == 0 {
		logger.Printf("auth config for the '%s' registry is empty; using anonymous access\n", registry)
		return "", "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(containerRegistry.Auth)
	if err != nil {
		return "", "", fmt.Errorf("decode the '%s' registry auth config: %w", registry, err)
	}

	splits := strings.Split(string(decoded), ":")
	if len(splits) < 2 {
		return "", "", fmt.Errorf("the '%s' registry auth config is malformed, should have the format: 'user:password'", registry)
	}

	return splits[0], splits[1], nil
}

func (c *VulnerabilityCache) initDictionary(ctx context.Context) error {
	c.logger.Println("initialize BDU dictionary")
	if err := c.Renew(ctx); err != nil {
		c.logger.Println("failed to initialize BDU dictionary")
		return fmt.Errorf("renew the dictionary: %w", err)
	}

	return nil
}

func (c *VulnerabilityCache) Renew(ctx context.Context) error {
	c.logger.Println("download BDU image")

	// set target repository
	repo, err := remote.NewRepository(c.config.registry + "/" + c.config.repository)
	if err != nil {
		return fmt.Errorf("new repository: %w", err)
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
		Credential: auth.StaticCredential(c.config.registry, auth.Credential{
			Username: c.config.user,
			Password: c.config.password,
		}),
	}

	// create oras in-memory storage
	store := memory.New()

	// copy the requested image from remote repository to oras in-memory storage and save its descriptor
	descriptor, err := oras.Copy(ctx, repo, c.config.tag, store, c.config.tag, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("copy BDU image to memory: %w", err)
	}

	// get successor descriptors of the descriptor
	successors, err := content.Successors(ctx, store, descriptor)
	if err != nil {
		return fmt.Errorf("get descriptors from BDU image: %w", err)
	}

	// iterate over descriptors to get the ones with relevant MediaType
	for _, desc := range successors {
		switch desc.MediaType {
		case tarGzMediaType:
			if err = c.processDescriptor(ctx, store, desc); err != nil {
				return fmt.Errorf("process tar archive: %w", err)
			}
		default:
			// skip
		}
	}

	return nil
}

func (c *VulnerabilityCache) processDescriptor(ctx context.Context, store *memory.Store, desc ocispec.Descriptor) error {
	tarGz, err := store.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("fetch tar archive: %w", err)
	}
	defer tarGz.Close()

	gzipReader, err := gzip.NewReader(tarGz)
	if err != nil {
		return fmt.Errorf("uncompress tar archive: %w", err)
	}

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("iterate over tar: %w", err)
		}

		if header.Name == bduDictionaryFilename {
			tempDict := &Dictionary{
				Data: make(map[string][]string),
			}

			if err = json.NewDecoder(tarReader).Decode(tempDict); err != nil {
				return fmt.Errorf("unmarshal BDU dictionary: %w", err)
			}

			if len(tempDict.Data) == 0 {
				return fmt.Errorf("dictionary is empty")
			}

			if tempDict.TS != c.dict.TS {
				c.mtx.Lock()
				c.dict.Data = tempDict.Data
				c.dict.TS = tempDict.TS
				c.mtx.Unlock()
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
	c.mtx.RLock()
	defer c.mtx.RUnlock()

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
