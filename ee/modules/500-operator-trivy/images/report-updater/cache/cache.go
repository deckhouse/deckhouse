/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	dockerConfigPath      = "/root/.docker/config.json"
	bduDictionaryFilename = "export.json"
	tarGzMediaType        = "application/deckhouse.io.bdu.layer.v1.tar+gzip"
)

type Cache interface {
	Get(string) ([]string, bool)
	Check() error
	RenewBduDictionary() error
}

type ContainerRegistry struct {
	Auth string `json:"auth"`
}

type DockerConfig struct {
	Auths map[string]ContainerRegistry `json:"auths"`
}

type RegistryConfig struct {
	registry   string
	repository string
	tag        string
	user       string
	password   string
}

type VulnerabilityDictionary struct {
	TS   time.Time           `json:"timestamp"`
	Data map[string][]string `json:"data"`
}

type VulnerabilityCache struct {
	logger *log.Logger

	Dictionary   VulnerabilityDictionary
	mu           sync.RWMutex
	sourceConfig RegistryConfig
}

func NewVulnerabilityCache(logger *log.Logger) (*VulnerabilityCache, error) {
	var dockerConfig DockerConfig
	var containerRegistry ContainerRegistry
	var ok bool

	image := os.Getenv("DICTIONARY_OCI_IMAGE")
	if len(image) == 0 {
		return nil, fmt.Errorf("DICTIONARY_OCI_IMAGE env not set")
	}

	if len(strings.Split(image, ":")) < 2 || len(strings.Split(image, "/")) < 2 {
		return nil, fmt.Errorf("dictionary image env value seems to be malformed, should adhere to the following format: '<registry>/<repository>:<tag>'")
	}
	registry := strings.Split(image, "/")[0]

	dockerConfigFile, err := os.Open(dockerConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open docker config.json: %w", err)
	}
	defer dockerConfigFile.Close()

	err = json.NewDecoder(dockerConfigFile).Decode(&dockerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal docker config.json: %w", err)
	}

	if containerRegistry, ok = dockerConfig.Auths[registry]; !ok {
		return nil, fmt.Errorf("failed to find auth config for bdu registry %s", registry)
	}

	if len(containerRegistry.Auth) == 0 {
		return nil, fmt.Errorf("bdu registry auth config is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(containerRegistry.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bdu registry auth config: %w", err)
	}

	auth := string(decoded)

	if len(strings.Split(auth, ":")) < 2 {
		return nil, fmt.Errorf("bdu registry auth config seems to be malformed, should have the following format: 'user:password'")
	}

	d := &VulnerabilityCache{
		logger: logger,
		Dictionary: VulnerabilityDictionary{
			Data: make(map[string][]string),
		},
		sourceConfig: RegistryConfig{
			registry:   registry,
			repository: strings.Split(image, ":")[0],
			tag:        strings.Split(image, ":")[1],
			user:       strings.Split(auth, ":")[0],
			password:   strings.Split(auth, ":")[1],
		},
	}

	err = d.initDictionary()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// think about healthz check
func (c *VulnerabilityCache) Check() error {
	if len(c.Dictionary.Data) == 0 {
		return fmt.Errorf("BDU dictionary empty")
	}
	return nil
}

func (c *VulnerabilityCache) getDataFromImageDescriptors() error {
	//download
	c.logger.Println("downloading BDU image")
	//create oras in-memory storage
	store := memory.New()
	ctx := context.Background()

	//set target repository
	repo, err := remote.NewRepository(c.sourceConfig.repository)
	if err != nil {
		return err
	}

	//set repository auth
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.DefaultCache,
		Credential: auth.StaticCredential(c.sourceConfig.registry, auth.Credential{
			Username: c.sourceConfig.user,
			Password: c.sourceConfig.password,
		}),
	}

	//copy requested image from remote repository to oras in-memory storage and save its descriptor
	descriptor, err := oras.Copy(ctx, repo, c.sourceConfig.tag, store, c.sourceConfig.tag, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("renewing BDU failed: couldn't copy BDU image to memory: %w", err)
	}

	//get successor descriptors of the descriptor
	successors, err := content.Successors(ctx, store, descriptor)
	if err != nil {
		return fmt.Errorf("renewing BDU failed: couldn't get descriptors from BDU image: %w", err)
	}

	//iterate over descriptors to get the ones with relevant MediaType
	for _, descriptor := range successors {
		switch descriptor.MediaType {
		case tarGzMediaType:
			err = c.processTarGzMedia(store, descriptor, ctx)
			if err != nil {
				fmt.Errorf("renewing BDU failed: couldn't process tar archive: %w", err)
			}
		default:
			//skip
		}
	}

	return nil
}

func (c *VulnerabilityCache) processTarGzMedia(store *memory.Store, descriptor ocispec.Descriptor, ctx context.Context) error {
	tarGz, err := store.Fetch(ctx, descriptor)
	if err != nil {
		fmt.Errorf("renewing BDU failed: couldn't fetch tar archive: %w", err)
	}
	defer tarGz.Close()

	uncompressedStream, err := gzip.NewReader(tarGz)
	if err != nil {
		return fmt.Errorf("renewing BDU failed: couldn't uncompress tar archive %w", err)
	}

	tarReader := tar.NewReader(uncompressedStream)
	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("renewing BDU failed: couldn't iterate over tar: %w", err)
		}

		if header.Name == bduDictionaryFilename {
			tempDict := &VulnerabilityDictionary{
				Data: make(map[string][]string),
			}

			err = json.NewDecoder(tarReader).Decode(&tempDict)
			if err != nil {
				return fmt.Errorf("renewing BDU failed: couldn't unmarshal bdu dictionary: %w", err)
			}

			if len(tempDict.Data) == 0 {

				return fmt.Errorf("renewing BDU failed: dictionary is empty")
			}
			if tempDict.TS != c.Dictionary.TS {
				c.mu.Lock()
				defer c.mu.Unlock()

				c.Dictionary.Data = tempDict.Data
				c.Dictionary.TS = tempDict.TS
				c.logger.Printf("BDU dictionary dated %v has been applied", c.Dictionary.TS)
			} else {
				c.logger.Printf("BDU dictionary is up to date (ts: %s)", c.Dictionary.TS)
			}

			break
		}
	}

	return nil
}

func (c *VulnerabilityCache) RenewBduDictionary() error {
	err := c.getDataFromImageDescriptors()
	if err != nil {
		return fmt.Errorf("renewing BDU failed: couldn't get data from image descriptors: %w", err)
	}

	return nil
}

func (c *VulnerabilityCache) initDictionary() error {
	c.logger.Println("initializing BDU dictionary")
	err := c.RenewBduDictionary()
	if err != nil {
		c.logger.Println("failed to initialize BDU dictionary")
		return err
	}

	return nil
}

func (c *VulnerabilityCache) Get(vulnerability string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.Dictionary.Data[vulnerability]
	return entry, ok
}
