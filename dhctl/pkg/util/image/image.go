// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type dockerConfig struct {
	Auths map[string]authEntry `json:"auths"`
}

type authEntry struct {
	Auth          string `json:"auth"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	RegistryToken string `json:"registrytoken,omitempty"`
}

type RegistryConfig struct {
	registry string
	scheme   string
	username string
	password string
	ca       string
}

func (r *RegistryConfig) GetRegistry() string {
	return r.registry
}

func (r *RegistryConfig) GetScheme() string {
	return r.scheme
}

func (r *RegistryConfig) GetUsername() string {
	return r.username
}

func (r *RegistryConfig) GetPassword() string {
	return r.password
}

func (r *RegistryConfig) GetCA() string {
	return r.ca
}

func (r *RegistryConfig) SetCA(ca string) {
	r.ca = ca
}

func (c *dockerConfig) GetRegistries() []string {
	registries := []string{}
	for key := range c.Auths {
		registries = append(registries, key)
	}

	return registries
}

func RegistryConfigFromDockerConfig(dc *dockerConfig, scheme, registry string) (*RegistryConfig, error) {
	if scheme != "HTTPS" && scheme != "HTTP" {
		return nil, fmt.Errorf("scheme must be HTTP or HTTPS")
	}
	baseRegistry := registry
	parts := strings.Split(registry, "/")
	if len(parts) > 0 {
		baseRegistry = parts[0]
	}

	_, ok := dc.Auths[baseRegistry]
	if !ok {
		return nil, fmt.Errorf("docker config doesn't contains %s registry credentials", registry)
	}
	rc := &RegistryConfig{
		scheme:   scheme,
		registry: registry,
		username: dc.Auths[baseRegistry].Username,
		password: dc.Auths[baseRegistry].Password,
	}

	if dc.Auths[baseRegistry].Auth != "" {
		parts := []byte(dc.Auths[baseRegistry].Auth)
		decoded, err := base64.StdEncoding.DecodeString(string(parts))
		if err != nil {
			return nil, fmt.Errorf("decoding auth field: %w", err)
		}
		cred := string(decoded)
		colonIdx := -1
		for i, ch := range cred {
			if ch == ':' {
				colonIdx = i
				break
			}
		}
		if colonIdx == -1 {
			return nil, fmt.Errorf("invalid auth format, missing ':'")
		}
		rc.username = cred[:colonIdx]
		rc.password = cred[colonIdx+1:]
	}

	return rc, nil
}

func NewRegistryConfig(scheme, registry, username, password, ca string) (*RegistryConfig, error) {
	if scheme != "HTTPS" && scheme != "HTTP" {
		return nil, fmt.Errorf("scheme must be HTTP or HTTPS")
	}

	return &RegistryConfig{
		registry: registry,
		scheme:   scheme,
		username: username,
		password: password,
		ca:       ca,
	}, nil
}

func DecodeDockerConfig(b64 string) (*dockerConfig, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 dockerconfig: %w", err)
	}
	return ParseDockerConfig(decoded)
}

func ParseDockerConfig(decoded []byte) (*dockerConfig, error) {
	var cfg dockerConfig
	if err := json.Unmarshal(decoded, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling dockerconfig JSON: %w", err)
	}
	return &cfg, nil
}

func authFromRegistry(cfg *RegistryConfig, registry string) (authn.Authenticator, error) {
	registryWithScheme := strings.ToLower(cfg.scheme) + "://" + registry
	_, err := url.Parse(registryWithScheme)
	if err != nil {
		return nil, fmt.Errorf("parsing registry URL %q: %w", registry, err)
	}

	if cfg.username != "" && cfg.password != "" {
		basic := authn.Basic{Username: cfg.username, Password: cfg.password}
		return &basic, nil
	}

	return authn.Anonymous, nil
}

func hashFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", cerr)
		}
	}()

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to copy file content to hash: %w", err)
	}

	hashInBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)

	return hashString, nil
}

func getHash(digest, dstPath string) (string, error) {
	path := filepath.Join(dstPath, "images_hashs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file %s: %w", path, err)
	}
	var hashs map[string]interface{}
	err = json.Unmarshal(data, &hashs)
	if err != nil {
		return "", fmt.Errorf("unmarshalling json: %w", err)
	}
	hash, ok := hashs[digest]
	if ok {
		return hash.(string), nil
	}

	return "", nil
}

func saveHash(digest, hash, dstPath string) error {
	path := filepath.Join(dstPath, "images_hashs.json")
	data, err := os.ReadFile(path)
	hashs := make(map[string]string)
	if err != nil {
		if err1 := os.RemoveAll(path); err1 != nil {
			return err1
		}
		hashs[digest] = hash
		bytes, err1 := json.Marshal(hashs)
		if err1 != nil {
			return err1
		}
		if err1 = os.WriteFile(path, bytes, 0o644); err1 != nil {
			return err1
		}
		return nil
	}

	if err = json.Unmarshal(data, &hashs); err != nil {
		return fmt.Errorf("unmarshalling json: %w", err)
	}

	hashs[digest] = hash
	bytes, err := json.Marshal(hashs)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path, bytes, 0o644); err != nil {
		return err
	}

	return nil
}

func pullImage(ctx context.Context, ref name.Reference, opts []remote.Option, digest, dstPath, cacheDir string) (v1.Image, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create cache directory %s: %w\n", cacheDir, err)
	}
	layersCache := cache.NewFilesystemCache(cacheDir)
	img, err := remote.Image(ref, opts...)
	if err != nil {
		return img, fmt.Errorf("pulling image: %w", err)
	}
	cached := cache.Image(img, layersCache)

	checksum, err := saveImageAsTarGz(ctx, ref.String(), filepath.Join(dstPath, digest), cached)
	if err != nil {
		return cached, fmt.Errorf("saving tar.gz: %w", err)
	}

	log.DebugF("checksum: %s\n", checksum)
	if err = saveHash(digest, checksum, dstPath); err != nil {
		return cached, fmt.Errorf("saving checksum to file: %w", err)
	}

	return cached, nil
}

func getEstimatedTarSize(img v1.Image) (int64, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, l := range manifest.Layers {
		total += l.Size
	}

	total += manifest.Config.Size
	return total, nil
}

func saveImageAsTarGz(ctx context.Context, imageRef string, outPath string, img v1.Image) (string, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	var bar *mpb.Bar
	var p *mpb.Progress
	if input.IsTerminal() {
		total, err := getEstimatedTarSize(img)
		if err != nil {
			return "", fmt.Errorf("getting image size %q: %w", imageRef, err)
		}

		p = mpb.New(mpb.WithWidth(64))
		bar = p.New(int64(total),
			mpb.BarStyle(),
			mpb.PrependDecorators(
				decor.Name("downloading image", decor.WC{C: decor.DindentRight | decor.DextraSpace}),
				decor.OnComplete(decor.AverageETA(decor.ET_STYLE_GO), "done"),
			),
			mpb.AppendDecorators(decor.Percentage()),
		)
	}

	tmpTar, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating tar file: %w", err)
	}

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tmpTar, hasher)
	proxyWriter := multiWriter
	if input.IsTerminal() {
		proxyWriter = bar.ProxyWriter(multiWriter)
	}
	if err := tarball.Write(ref, img, proxyWriter); err != nil {
		tmpTar.Close()
		return "", fmt.Errorf("writing tarball: %w", err)
	}
	tmpTar.Close()
	if input.IsTerminal() {
		p.Wait()
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	return checksum, nil
}

func getOptsFromRegistryConfig(ref name.Reference, cfg *RegistryConfig) ([]remote.Option, error) {
	var opts []remote.Option
	registry := ref.Context().RegistryStr()
	auth, err := authFromRegistry(cfg, registry)
	if err != nil {
		return opts, fmt.Errorf("creating authenticator: %w", err)
	}
	opts = append(opts, remote.WithAuth(auth))
	if cfg.ca != "" {
		tlsCfg, err := loadCustomCA([]byte(cfg.ca))
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{
			TLSClientConfig: tlsCfg,
		}
		opts = append(opts, remote.WithTransport(transport))
	}

	return opts, nil
}

func DownloadAndUnpackImage(ctx context.Context, imageRef, destDir, cacheDir string, regConfig RegistryConfig) error {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	imgName := ref.Identifier()
	img, err := tryToRestoreLocalImage(destDir, imgName)
	if err == nil {
		return extractImage(img, destDir)
	}
	log.DebugF("Could not use local image. Reason: %s\n", err.Error())

	opts, err := getOptsFromRegistryConfig(ref, &regConfig)
	if err != nil {
		return err
	}

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return fmt.Errorf("getting manifest descriptor for %q: %w", ref.String(), err)
	}
	log.DebugF("hash: %s\n", desc.Digest.String())

	img, err = pullImage(ctx, ref, opts, desc.Digest.String(), destDir, cacheDir)
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", imageRef, err)
	}

	return extractImage(img, destDir)
}

func extractImage(img v1.Image, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating destination directory %q: %w", destDir, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("getting layers: %w", err)
	}

	for _, layer := range layers {
		err = extractLayer(layer, destDir)
		if err != nil {
			return fmt.Errorf("extracting layer: %w", err)
		}
	}

	return os.RemoveAll(filepath.Join(destDir, ".werf"))
}

func extractLayer(layer v1.Layer, destDir string) error {
	rc, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("opening compressed layer: %w", err)
	}
	defer rc.Close()

	peek := make([]byte, 2)
	if _, err := io.ReadFull(rc, peek); err != nil {
		return fmt.Errorf("reading layer header: %w", err)
	}

	combined := io.MultiReader(bytes.NewReader(peek), rc)

	return processLayer(combined, peek, destDir)
}

func processLayer(r io.Reader, peek []byte, destDir string) error {
	var tarReader *tar.Reader
	if peek[0] == 0x1f && peek[1] == 0x8b {
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else {
		tarReader = tar.NewReader(r)
	}

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		targetPath := filepath.Join(destDir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(targetPath, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("creating directory %q: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent dir for %q: %w", targetPath, err)
			}
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return fmt.Errorf("creating file %q: %w", targetPath, err)
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return fmt.Errorf("copying file %q: %w", targetPath, err)
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("removing symlink %q: %w", targetPath, err)
			}
			if err := os.Symlink(hdr.Linkname, targetPath); err != nil {
				return fmt.Errorf("creating symlink %q -> %q: %w", targetPath, hdr.Linkname, err)
			}
		case tar.TypeLink:
			linkTarget := filepath.Join(destDir, hdr.Linkname)
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("removing hard link %q -> %q: %w", targetPath, linkTarget, err)
			}
			if err := os.Link(linkTarget, targetPath); err != nil {
				return fmt.Errorf("creating hard link %q -> %q: %w", targetPath, linkTarget, err)
			}
		default:
		}
	}
	return nil
}

func tryToRestoreLocalImage(imgName, destDir string) (v1.Image, error) {
	filename := filepath.Join(destDir, imgName)
	f, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("could not close file %s: %w", filename, err)
	}

	fileChecksum, err := hashFileSHA256(filename)
	if err != nil {
		return nil, fmt.Errorf("could not calculate file checksum %s: %w", filename, err)
	}

	storedChecksum, err := getHash(imgName, destDir)
	if err != nil {
		return nil, fmt.Errorf("could not get checksum %s from file: %w", filename, err)
	}

	if fileChecksum != storedChecksum {
		return nil, fmt.Errorf("stored checksum must be the same as file checksum")
	}

	return restoreImageFromTarGz(filename, nil)
}

func restoreImageFromTarGz(path string, tag *name.Tag) (v1.Image, error) {
	img, err := tarball.ImageFromPath(path, tag)
	if err != nil {
		return nil, fmt.Errorf("parsing tarball: %w", err)
	}

	return img, nil
}

func loadCustomCA(ca []byte) (*tls.Config, error) {
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, fmt.Errorf("failed to parse CA PEM")
	}

	return &tls.Config{
		RootCAs: certPool,
	}, nil
}
