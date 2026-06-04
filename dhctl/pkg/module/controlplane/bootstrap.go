// Copyright 2026 Flant JSC
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
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"gopkg.in/yaml.v2"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	tlsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/tls"
)

const (
	privKeyFilename = "signature-private.jwk"
	pubKeyFilename  = "signature-public.jwks"
	configFilename  = "encryption-config.yaml"

	signaturePath        = app.NodeDeckhouseDirectoryPath + `/signature`
	kubernetesConfigPath = `/etc/kubernetes`
	kubernetesPkiPath    = kubernetesConfigPath + `/pki`
)

var (
	createSigDirDefaultOpts = retry.AttemptsWithWaitOpts(10, 10*time.Second)
)

type LoopsParams struct {
	CreateSigDir retry.Params
}

type ModuleSettings interface {
	SignatureMode() (string, error)
}

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
	settings       ModuleSettings
	node           libcon.Interface
	loggerProvider log.LoggerProvider
	timeNow        TimeFunc
	dirPathPrefix  string
	loopsParams    LoopsParams
}

func NewBootstrapPreparator(settings ModuleSettings, node libcon.Interface, loggerProvider log.LoggerProvider) *BootstrapPreparator {
	return &BootstrapPreparator{
		settings:       settings,
		node:           node,
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

func (p *BootstrapPreparator) WithLoopsParams(l LoopsParams) *BootstrapPreparator {
	p.loopsParams = l

	return p
}

func (p *BootstrapPreparator) WithDirPrefix(pr string) *BootstrapPreparator {
	p.dirPathPrefix = pr

	return p
}

func (p *BootstrapPreparator) PrepareModule(ctx context.Context) error {
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
		return p.generateKeysAndUpload(ctx, signatureMode)
	})
}

func (p *BootstrapPreparator) Module() string {
	return moduleName
}

func (p *BootstrapPreparator) generateKeysAndUpload(ctx context.Context, signatureMode string) error {
	p.loggerProvider().DebugF("Got signature mode: %s. Start preparing control-plane signature", signatureMode)

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

	return p.uploadSignatureFiles(ctx, &signature{
		keys:   keys,
		config: encConfig,
	})
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

	logger.DebugF("Generate key for signature")
	pubKeyCert, err := x509.ParseCertificate(certs.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("Cannot parse certificate: %w", err)
	}

	pubKey, ok := pubKeyCert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Unexpected public key type %T, expected ed25519.PublicKey", pubKeyCert.PublicKey)
	}

	logger.DebugF("Generate certificate for signature")
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
	p.loggerProvider().DebugF("Generate encryption config")

	config := EncryptionConfiguration{
		APIVersion: "apiserver.config.k8s.io/v1",
		Kind:       "EncryptionConfiguration",
		Signature: Signature{
			PrivKeyPath: fs.JoinLinux(kubernetesPkiPath, privKeyFilename),
			PubKeyPath:  fs.JoinLinux(kubernetesPkiPath, pubKeyFilename),
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

	full = append(full, signaturePath)
	full = append(full, parts...)

	path := fs.JoinLinux(full...)

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

func (p *BootstrapPreparator) createSignatureDir(ctx context.Context) error {
	signaturePath := p.signaturePath()

	logger := p.loggerProvider()

	logger.DebugF("Create signature dir %s", signaturePath)

	loopParams := retry.SafeCloneOrNewParams(p.loopsParams.CreateSigDir, createSigDirDefaultOpts...).
		Clone(
			retry.WithName("Prepare %s", signaturePath),
			retry.WithLogger(logger),
		)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		cmd := p.node.Command("sh", "-c", fmt.Sprintf("umask 0022 ; mkdir -p -m 1777 %s", signaturePath))
		cmd.Sudo(ctx)
		if err := cmd.Run(ctx); err != nil {
			return fmt.Errorf("Cannot create signature dir: mkdir -p -m 1777 %s: %w", signaturePath, err)
		}

		return nil
	})
}

type signature struct {
	*keys
	config []byte
}

func (p *BootstrapPreparator) uploadSignatureFiles(ctx context.Context, sig *signature) error {
	if err := p.createSignatureDir(ctx); err != nil {
		return err
	}

	p.loggerProvider().DebugF("Upload signature files")
	files := map[string][]byte{
		p.signaturePath(privKeyFilename): sig.privateKeyJSON,
		p.signaturePath(pubKeyFilename):  sig.jwksJSON,
		p.signaturePath(configFilename):  sig.config,
	}

	for filePath, content := range files {
		err := p.node.File().UploadBytes(ctx, content, filePath)
		if err != nil {
			return fmt.Errorf("Cannot create file %s: %w", filePath, err)
		}
	}

	return nil
}
