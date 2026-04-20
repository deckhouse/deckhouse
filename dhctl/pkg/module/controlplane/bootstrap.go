// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/go-jose/go-jose/v4"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	tlsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/tls"
)

const (
	privKeyFilename = "signature-private.jwk"
	pubKeyFilename  = "signature-public.jwks"

	SignaturePath        = app.NodeDeckhouseDirectoryPath + `/signature`
	KubernetesConfigPath = `/etc/kubernetes`
	KubernetesPkiPath    = KubernetesConfigPath + `/pki`
	DeckhousePath        = KubernetesConfigPath + `/deckhouse`
)

type EncryptionConfiguration struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Signature  Signature `yaml:"signature"`
}

type Signature struct {
	PrivKeyPath string `yaml:"privKeyPath"`
	PubKeyPath  string `yaml:"pubKeyPath"`
	Mode        string `yaml:"mode"`
}

type TimeFunc func() time.Time

type BootstrapPreparator struct {
	settings       *SettingsExtractor
	nodeInterface  node.Interface
	loggerProvider log.LoggerProvider
	timeNow        TimeFunc
	dirPathPrefix  string
}

func NewBootstrapPreparator(settings *SettingsExtractor, loggerProvider log.LoggerProvider, nodeInterface node.Interface) *BootstrapPreparator {
	return &BootstrapPreparator{
		settings:       settings,
		nodeInterface:  nodeInterface,
		loggerProvider: loggerProvider,
		timeNow:        time.Now,
	}
}

func (p *BootstrapPreparator) WithTimeFunc(f TimeFunc) *BootstrapPreparator {
	if f != nil {
		p.timeNow = f
	}

	return p
}

func (p *BootstrapPreparator) WithDirPrefix(pr string) *BootstrapPreparator {
	p.dirPathPrefix = pr

	return p
}

func (p *BootstrapPreparator) Prepare(ctx context.Context) error {
	logger := p.loggerProvider()

	signatureMode, err := p.settings.SignatureMode()
	if err != nil {
		return err
	}

	if signatureMode == NoSignatureMode {
		logger.DebugF("Provide no signature mode. Skip prepare signature")
		return nil
	}

	return logger.Process(log.ProcessBootstrap, "Configure signature certificates for control-plane", func() error {
		logger.DebugF("Got signature mode: %s. Start preparing control-plane signature", signatureMode)

		certs, err := tlsutils.GenerateCertificate("signature", "apiserver", tlsutils.CertKeyTypeED25519, 365)
		if err != nil {
			return fmt.Errorf("Cannot generate tls certificate: %w", err)
		}

		keys, err := p.generateKeys(certs, p.timeNow())
		if err != nil {
			return err
		}

		encConfig, err := p.encryptionConfig(signatureMode)
		if err != nil {
			return err
		}

		if err := p.createSignatureDir(ctx); err != nil {
			return err
		}

	})

	log.InfoF("Upload files\n")
	files := map[string][]byte{
		filepath.Join(filepath.Join(folderPathPrefix, SignaturePath), privKeyFilename):          privJSON,
		filepath.Join(filepath.Join(folderPathPrefix, SignaturePath), pubKeyFilename):           jwksJSON,
		filepath.Join(filepath.Join(folderPathPrefix, SignaturePath), "encryption-config.yaml"): yamlData,
	}
	for filePath, content := range files {
		err = nodeInterface.File().UploadBytes(ctx, content, filePath)
		if err != nil {
			return fmt.Errorf("create file %s: %v", filePath, err)
		}
	}
	return nil

}

type keys struct {
	privateKeyJSON []byte
	jwksJSON       []byte
}

func (p *BootstrapPreparator) generateKeys(certs *tls.Certificate, now time.Time) (*keys, error) {
	privKey, ok := certs.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unexpected key type %T, expected ed25519.PrivateKey", certs.PrivateKey)
	}

	logger := p.loggerProvider()

	timeNow := now.Format("2006-01-02 15:04")
	privJWK := jose.JSONWebKey{
		Key:       privKey,
		KeyID:     timeNow,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}

	privJSON, err := json.Marshal(privJWK)
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal private key JWK: %w", err)
	}

	logger.InfoF("Generate key for signature")
	pubKeyCert, err := x509.ParseCertificate(certs.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("Cannot parse certificate: %w", err)
	}

	pubKey, ok := pubKeyCert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Unexpected public key type %T, expected ed25519.PublicKey", pubKeyCert.PublicKey)
	}

	logger.InfoF("Generate certificate for signature")
	pubJWK := jose.JSONWebKey{
		Key:          pubKey,
		KeyID:        timeNow,
		Algorithm:    string(jose.EdDSA),
		Use:          "sig",
		Certificates: []*x509.Certificate{pubKeyCert},
	}
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{pubJWK},
	}

	jwksJSON, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal JWKS: %w", err)
	}

	return &keys{
		privateKeyJSON: privJSON,
		jwksJSON:       jwksJSON,
	}, nil
}

func (p *BootstrapPreparator) encryptionConfig(signatureMode string) ([]byte, error) {
	p.loggerProvider().InfoF("Generate encryption config")
	config := EncryptionConfiguration{
		APIVersion: "apiserver.config.k8s.io/v1",
		Kind:       "EncryptionConfiguration",
		Signature: Signature{
			PrivKeyPath: fs.JoinLinux(KubernetesPkiPath, privKeyFilename),
			PubKeyPath:  fs.JoinLinux(KubernetesPkiPath, pubKeyFilename),
			Mode:        strings.ToLower(signatureMode),
		},
	}
	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal EncryptionConfiguration: %w", err)
	}

	return yamlData, nil
}

func (p *BootstrapPreparator) signaturePath(parts ...string) string {
	full := make([]string, 0)
	if p.dirPathPrefix != "" {
		full = append(full, p.dirPathPrefix)
	}
	full = append(full, SignaturePath)
	full = append(full, parts...)
	return fs.JoinLinux(full...)
}

func (p *BootstrapPreparator) createSignatureDir(ctx context.Context) error {
	signaturePath := p.signaturePath()

	p.loggerProvider().DebugF("Create signature dir %s", signaturePath)

	err := retry.NewLoop(fmt.Sprintf("Prepare %s", signaturePath), 30, 10*time.Second).RunContext(ctx, func() error {
		cmd := p.nodeInterface.Command("sh", "-c", fmt.Sprintf("umask 0022 ; mkdir -p -m 1777 %s", signaturePath))
		cmd.Sudo(ctx)
		if err := cmd.Run(ctx); err != nil {
			return fmt.Errorf("mkdir -p -m 1777 %s: %w", signaturePath, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("Cannot create signature dir %s: %w", signaturePath, err)
	}

	return nil
}

type signature struct {
	*keys
	config []byte
}

func (p *BootstrapPreparator) uploadSignatureFiles(ctx context.Context, sig *signature) error {
	
}
