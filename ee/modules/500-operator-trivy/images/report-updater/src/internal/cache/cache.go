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
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	dockerConfigDir       = "/root/.docker"
	dockerConfigFilename  = "config.json"
	bduDictionaryFilename = "export.json"
	tarGzMediaType        = "application/deckhouse.io.bdu.layer.v1.tar+gzip"
)

type VulnerabilityCache struct {
	logger *log.Logger
	dict   Dictionary
	mtx    sync.RWMutex
	config RegistryConfig
	// anonymous records whether the last lookup found no credentials, so that
	// the fallback is reported when it starts rather than on every renewal.
	anonymous bool
}

type Dictionary struct {
	TS   time.Time           `json:"timestamp"`
	Data map[string][]string `json:"data"`
}

type RegistryConfig struct {
	repository name.Repository
	tag        string
	configDir  string
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

	if err = useDockerConfig(dockerConfigDir); err != nil {
		return nil, err
	}

	cache := &VulnerabilityCache{
		logger: logger,
		dict: Dictionary{
			Data: make(map[string][]string),
		},
		config: RegistryConfig{
			repository: ref.Context(),
			tag:        ref.Identifier(),
			configDir:  dockerConfigDir,
		},
	}

	if err = cache.initDictionary(ctx); err != nil {
		return nil, fmt.Errorf("init dictionary: %w", err)
	}

	return cache, nil
}

// useDockerConfig points the go-containerregistry keychain at the docker config
// mounted into the pod.
//
// The keychain reads $HOME/.docker/config.json first and falls back to
// $DOCKER_CONFIG, but the image is distroless and the pod runs as the deckhouse
// user, so $HOME is nothing to rely on. Both branches end up loading
// $DOCKER_CONFIG when it is set, which makes the lookup deterministic.
func useDockerConfig(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, dockerConfigFilename)); err != nil {
		return fmt.Errorf("docker config in %s: %w", dir, err)
	}

	return os.Setenv("DOCKER_CONFIG", dir)
}

// credential resolves the credentials for the BDU repository from the mounted
// docker config.
//
// The lookup is delegated to go-containerregistry instead of reading the file
// by hand. An entry of a kubernetes.io/dockerconfigjson secret is valid with
// either the `auth` field or a `username`/`password` pair, it may hold an
// identity or a registry token, and its key may carry a scheme or a repository
// path. Kubelet accepts every one of those shapes when it pulls images, so
// credentials that work for imagePullSecrets have to work here too.
//
// The config file is re-read on every call, so credentials rotated in the
// secret are picked up on the next dictionary renewal without a restart.
func (c *VulnerabilityCache) credential() (auth.Credential, error) {
	repository := c.config.repository

	authenticator, err := authn.DefaultKeychain.Resolve(repository)
	if err != nil {
		// The keychain decodes the `auth` field of every entry and rejects the
		// whole file on the first one it cannot read, naming neither the entry
		// nor its registry. Name it, so that a broken entry of an unrelated
		// registry does not turn into an opaque crash loop.
		if broken := c.brokenAuthEntries(); len(broken) > 0 {
			return auth.EmptyCredential, fmt.Errorf(
				"resolve credentials for '%s': auth of %v is not base64(user:password): %w",
				repository, broken, err,
			)
		}

		return auth.EmptyCredential, fmt.Errorf("resolve credentials for '%s': %w", repository, err)
	}

	authConfig, err := authenticator.Authorization()
	if err != nil {
		return auth.EmptyCredential, fmt.Errorf("read credentials for '%s': %w", repository, err)
	}

	// Take the fields as they are and never marshal AuthConfig back to JSON:
	// its MarshalJSON recomputes `auth` from the username and the password,
	// which turns a token-only entry into base64(":").
	// See https://github.com/google/go-containerregistry/issues/1864.
	credential := auth.Credential{
		Username:     authConfig.Username,
		Password:     authConfig.Password,
		RefreshToken: authConfig.IdentityToken,
		AccessToken:  authConfig.RegistryToken,
	}

	// Report the fallback when it starts, not on every renewal: a registry that
	// needs no authentication is a legitimate setup, and a warning per renewal
	// would only bury the renewal errors that follow a real credential loss.
	// The renewal loop is the only caller, so the flag needs no lock.
	if credential == auth.EmptyCredential {
		if !c.anonymous {
			c.logger.Printf(
				"WARNING: no credentials for '%s' in %s (it holds %v); using anonymous access",
				repository, filepath.Join(c.config.configDir, dockerConfigFilename), c.dockerConfigHosts(),
			)
		}
		c.anonymous = true
	} else {
		c.anonymous = false
	}

	return credential, nil
}

// dockerConfigHosts returns the sorted keys of the `auths` object of the
// mounted docker config. It only feeds the log message that reports missing
// credentials: naming the keys that are in the file turns "which registry did
// it look for" into a single line of the log.
func (c *VulnerabilityCache) dockerConfigHosts() []string {
	config, err := c.readDockerConfig()
	if err != nil {
		return nil
	}

	return slices.Sorted(maps.Keys(config.Auths))
}

// brokenAuthEntries returns the registries of the mounted docker config whose
// `auth` field docker cannot decode, sorted. It applies docker's own rule:
// standard base64 with padding, holding a `user:password` pair with a
// non-empty user.
func (c *VulnerabilityCache) brokenAuthEntries() []string {
	config, err := c.readDockerConfig()
	if err != nil {
		return nil
	}

	broken := make([]string, 0, len(config.Auths))
	for registry, entry := range config.Auths {
		if entry.Auth == "" {
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			broken = append(broken, registry)
			continue
		}
		if user, _, ok := strings.Cut(string(decoded), ":"); !ok || user == "" {
			broken = append(broken, registry)
		}
	}
	slices.Sort(broken)

	return broken
}

type dockerConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func (c *VulnerabilityCache) readDockerConfig() (*dockerConfig, error) {
	file, err := os.Open(filepath.Join(c.config.configDir, dockerConfigFilename))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := new(dockerConfig)
	if err = json.NewDecoder(file).Decode(config); err != nil {
		return nil, err
	}

	return config, nil
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
	repo, err := remote.NewRepository(c.config.repository.Name())
	if err != nil {
		return fmt.Errorf("new repository: %w", err)
	}

	credential, err := c.credential()
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
		// A cache of its own per renewal: the shared DefaultCache would keep
		// serving a token issued for credentials that have since been rotated.
		Cache:      auth.NewCache(),
		Credential: auth.StaticCredential(c.config.repository.RegistryStr(), credential),
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
