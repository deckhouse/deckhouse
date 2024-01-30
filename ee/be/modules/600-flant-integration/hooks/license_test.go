/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: license ::", func() {

	Context("reading docker config", func() {
		It("Parses full config", func() {
			registry := rand.String(8)
			auth := getConfig()
			dockerCfg := prepareDockerConfig(auth, registry)

			lic, err := parseLicenseKeyFromDockerCredentials(dockerCfg, registry)

			Expect(err).To(BeNil())
			Expect(lic).To(Equal(auth.Password))
		})

		It("Prioritizes `password` field", func() {
			registry := rand.String(8)
			auth := getConfig()
			auth.Auth = ""
			dockerCfg := prepareDockerConfig(auth, registry)

			lic, err := parseLicenseKeyFromDockerCredentials(dockerCfg, registry)

			Expect(err).To(BeNil())
			Expect(lic).To(Equal(auth.Password))
		})

		It("Falls back to parsing `auth` field", func() {
			registry := rand.String(8)
			auth := getConfig()
			password := auth.Password
			dockerCfg := prepareDockerConfig(auth, registry)

			lic, err := parseLicenseKeyFromDockerCredentials(dockerCfg, registry)

			Expect(err).To(BeNil())
			Expect(lic).To(Equal(password))
		})

		It("Fails with improper registry", func() {
			dockerCfg := prepareDockerConfig(getConfig(), rand.String(8))

			_, err := parseLicenseKeyFromDockerCredentials(dockerCfg, rand.String(10))

			Expect(err).ToNot(BeNil())
		})

		It("Fails with improper `auth` field", func() {
			registry := rand.String(8)
			auth := getConfig()
			auth.Password = ""
			auth.Auth = base64.StdEncoding.EncodeToString([]byte(auth.Username))
			dockerCfg := prepareDockerConfig(auth, registry)

			_, err := parseLicenseKeyFromDockerCredentials(dockerCfg, registry)

			Expect(err).ToNot(BeNil())
		})

		It("Fails with empty credentials", func() {
			registry := rand.String(8)
			auth := getConfig()
			auth.Password = ""
			auth.Auth = ""
			dockerCfg := prepareDockerConfig(auth, registry)

			_, err := parseLicenseKeyFromDockerCredentials(dockerCfg, registry)

			Expect(err).ToNot(BeNil())
		})

		It("Tolerates newline character in password", func() {
			registry := rand.String(8)
			auth := getConfig()
			passwordWithNoSpaces := auth.Password
			auth.Password += "\n"
			auth.Auth = base64.StdEncoding.EncodeToString([]byte(auth.Username + ":" + auth.Password))
			dockerCfg := prepareDockerConfig(auth, registry)

			lic, err := parseLicenseKeyFromDockerCredentials(dockerCfg, registry)

			Expect(err).To(BeNil())
			Expect(lic).To(Equal(passwordWithNoSpaces))
		})
	})

	Context("get license key", func() {
		const (
			testLicenseKey = "TeStLiCeNsE"
			testRepo       = "test.repo"
		)

		var (
			revokedCMManifest = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ` + revokedCMName + `
  namespace: ` + revokedCMNamespace + "\n"
		)

		f := HookExecutionConfigInit(`{"global":{}, "flantIntegration":{"internal": {}}}`, `{"flantIntegration":{}}`)

		Context("with license key in configuration", func() {

			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ConfigValuesSet(licenseKeyPath, testLicenseKey)
				f.RunHook()
			})

			It("should set internal value from configuration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(internalLicenseKeyPath).String()).To(Equal(testLicenseKey))
			})
		})

		Context("with license key in docker config", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ConfigValuesSet(licenseKeyPath, "")
				f.ValuesSet(globalRegistryPath, "test.repo")
				f.ValuesSet(globalRegistryDockercfg, getDockerConfig(testRepo, testLicenseKey))
				f.RunHook()
			})

			It("should set internal value from configuration", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(internalLicenseKeyPath).String()).To(Equal(testLicenseKey))
			})
		})

		Context("with revoked config map", func() {
			BeforeEach(func() {
				f.KubeStateSet(revokedCMManifest)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ConfigValuesSet(licenseKeyPath, "")
				f.ValuesSet(globalRegistryPath, "test.repo")
				f.ValuesSet(globalRegistryDockercfg, getDockerConfig(testRepo, testLicenseKey))
				f.ValuesSet(internalLicenseKeyPath, testLicenseKey)
				f.RunHook()
			})

			It("should remove internal value", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(internalLicenseKeyPath).String()).To(BeEmpty())
			})
		})

	})
})

func getConfig() authn.AuthConfig {
	username := rand.String(8)
	password := rand.String(17)

	return authn.AuthConfig{
		Username: username,
		Password: password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
	}
}

func prepareDockerConfig(a authn.AuthConfig, registry string) []byte {
	c := dockerFileConfig{Auths: map[string]authn.AuthConfig{
		registry: a,
	}}
	j, _ := json.Marshal(c)
	return j
}

func getDockerConfig(repo string, licenseKey string) string {
	auth := authn.AuthConfig{
		Username: "user",
		Password: licenseKey,
		Auth:     base64.StdEncoding.EncodeToString([]byte("user:" + licenseKey)),
	}

	return base64.StdEncoding.EncodeToString(prepareDockerConfig(auth, repo))
}
