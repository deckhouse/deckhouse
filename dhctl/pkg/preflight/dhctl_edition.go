// Copyright 2023 Flant JSC
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

package preflight

// import (
// 	"context"
// 	"crypto/tls"
// 	"crypto/x509"
// 	"encoding/base64"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"net/http"
// 	"strings"

// 	"github.com/google/go-containerregistry/pkg/authn"
// 	"github.com/google/go-containerregistry/pkg/name"
// 	v1 "github.com/google/go-containerregistry/pkg/v1"
// 	"github.com/google/go-containerregistry/pkg/v1/remote"

// 	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
// 	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
// )

// const dhctlEditionMismatchError = "Your edition installer image does not match.\n" +
// 	"  The edition of the dhctl installer is - %s\n" +
// 	"  Editing images in registry is         - %s\n"

// // imageDescriptorProvider returns image manifest data, mainly image digest.
// type imageDescriptorProvider interface {
// 	ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error)
// }

// // remoteDescriptorProvider returns image manifest data from remote registry.
// type remoteDescriptorProvider struct{}

// func (remoteDescriptorProvider) ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error) {
// 	image, err := remote.Image(ref, opts...)
// 	if err != nil {
// 		return &v1.ConfigFile{}, err
// 	}
// 	return image.ConfigFile()
// }

// func (pc *Checker) CheckDhctlEdition(ctx context.Context) error {
// 	log.DebugLn("Checking if dhctl version is compatible with release to be installed")
// 	if app.AppVersion == "local" {
// 		log.DebugLn("dhctl version check is skipped for local builds")
// 		return nil
// 	}
// 	if app.PreflightSkipDeckhouseEditionCheck {
// 		log.WarnLn("Dhctl compatibility check is skipped")
// 		return nil
// 	}

// 	imageConfig, err := pc.getDeckhouseImageConfig(ctx)
// 	if err != nil {
// 		return fmt.Errorf("Cannot fetch deckhouse image config: %w.", err)
// 	}
// 	if imageConfig == nil ||
// 		imageConfig.Config.Labels == nil ||
// 		imageConfig.Config.Labels["io.deckhouse.edition"] != app.AppEdition {
// 		return errors.New(fmt.Sprintf(dhctlEditionMismatchError, app.AppEdition, imageConfig.Config.Labels["io.deckhouse.edition"]))
// 	}

// 	return nil
// }

// func (pc *Checker) getDeckhouseImageConfig(ctx context.Context) (*v1.ConfigFile, error) {
// 	creds, err := pc.findRegistryAuthCredentials()
// 	if err != nil {
// 		return nil, fmt.Errorf("parse ClusterConfiguration.deckhouse.registryDockerCfg: %w", err)
// 	}

// 	var versionTagRef name.Reference
// 	if strings.ToLower(pc.metaConfig.Registry.Scheme) == "http" {
// 		versionTagRef, err = name.ParseReference(pc.installConfig.GetImage(true), name.Insecure)
// 	} else {
// 		versionTagRef, err = name.ParseReference(pc.installConfig.GetImage(true))
// 	}
// 	if err != nil {
// 		return nil, fmt.Errorf("parse image reference: %w", err)
// 	}

// 	client, err := pc.prepareTLS()

// 	config, err := pc.imageDescriptorProvider.ConfigFile(versionTagRef, remote.WithContext(ctx), remote.WithAuth(creds), remote.WithTransport(client.Transport))
// 	if err != nil {
// 		return nil, fmt.Errorf("pull deckhouse image ConfigFile from registry: %w", err)
// 	}

// 	return config, nil
// }

// func (pc *Checker) findRegistryAuthCredentials() (authn.Authenticator, error) {
// 	buf, err := base64.StdEncoding.DecodeString(pc.installConfig.Registry.DockerCfg)
// 	if err != nil {
// 		return nil, fmt.Errorf("decode dockerCfg: %w", err)
// 	}

// 	decodedDockerCfg := struct {
// 		Auths map[string]struct {
// 			Auth     string `json:"auth,omitempty"`
// 			User     string `json:"username,omitempty"`
// 			Password string `json:"password,omitempty"`
// 		} `json:"auths"`
// 	}{}
// 	if err := json.Unmarshal(buf, &decodedDockerCfg); err != nil {
// 		return nil, fmt.Errorf("decode dockerCfg: %w", err)
// 	}

// 	if decodedDockerCfg.Auths == nil {
// 		return authn.Anonymous, nil
// 	}
// 	registryAuth, hasRegistryCreds := decodedDockerCfg.Auths[pc.installConfig.Registry.Address]
// 	if !hasRegistryCreds {
// 		return authn.Anonymous, nil
// 	}

// 	if registryAuth.Auth != "" {
// 		return authn.FromConfig(authn.AuthConfig{
// 			Auth: registryAuth.Auth,
// 		}), nil
// 	}

// 	if registryAuth.User != "" && registryAuth.Password != "" {
// 		return authn.FromConfig(authn.AuthConfig{
// 			Username: registryAuth.User,
// 			Password: registryAuth.Password,
// 		}), nil
// 	}

// 	return authn.Anonymous, nil
// }

// func (pc *Checker) prepareTLS() (*http.Client, error) {
// 	client := &http.Client{}
// 	httpTransport := http.DefaultTransport.(*http.Transport).Clone()

// 	if strings.ToLower(pc.metaConfig.Registry.Scheme) == "http" || len(pc.metaConfig.Registry.CA) == 0 {
// 		client.Transport = httpTransport
// 		return client, nil
// 	}

// 	certPool := x509.NewCertPool()
// 	if ok := certPool.AppendCertsFromPEM([]byte(pc.metaConfig.Registry.CA)); !ok {
// 		return nil, fmt.Errorf("invalid cert in CA PEM")
// 	}

// 	httpTransport.TLSClientConfig = &tls.Config{
// 		RootCAs: certPool,
// 	}

// 	client.Transport = httpTransport

// 	return client, nil
// }
