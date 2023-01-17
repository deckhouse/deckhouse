/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"deckhouse-registry-webhook/internal/registryclient"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	admission "k8s.io/api/admission/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DockerConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

type ValidatingWebhook struct {
	addr           string
	tlsCertFile    string
	tlsKeyFile     string
	imageToCheck   string
	srv            *http.Server
	registryClient registryclient.RCInterface
}

func NewValidatingWebhook(addr, imageToCheck, tlsCertFile, tlsKeyFile string, registryClient registryclient.RCInterface) *ValidatingWebhook {
	return &ValidatingWebhook{
		tlsCertFile:    tlsCertFile,
		tlsKeyFile:     tlsKeyFile,
		imageToCheck:   imageToCheck,
		addr:           addr,
		registryClient: registryClient,
	}
}

func (vw *ValidatingWebhook) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		ctxShutDown := context.Background()
		ctxShutDown, cancel := context.WithTimeout(ctxShutDown, time.Second*5)
		defer func() {
			cancel()
		}()

		if vw.srv != nil {
			if err := vw.srv.Shutdown(ctxShutDown); err != nil {
				logrus.Fatalf("https server Shutdown Failed:%s", err)
			} else {
				logrus.Info("https server stopped")
			}
		}
	}()
	r := mux.NewRouter()
	r.PathPrefix("/validate").HandlerFunc(vw.ValidatingWebhook)
	vw.srv = &http.Server{
		Addr:    vw.addr,
		Handler: r,
	}

	// check if cert, key exist
	_, errKey := os.Stat(vw.tlsKeyFile)
	_, errCrt := os.Stat(vw.tlsCertFile)
	var err error
	if errKey == nil && errCrt == nil {
		logrus.Infof("serving https on %s", vw.addr)
		err = vw.srv.ListenAndServeTLS(vw.tlsCertFile, vw.tlsKeyFile)
	} else {
		logrus.Warnf("TLS cert and key files not found, serving http on %s", vw.addr)
		err = vw.srv.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return err
	}
	logrus.Info("app stopped")

	return nil
}

func (vw *ValidatingWebhook) checkURI(scheme, address, path string) error {
	if address == "" {
		return nil
	}
	if scheme == "" {
		scheme = "https"
	}
	uri := fmt.Sprintf("%s://%s%s", scheme, address, path)
	_, err := url.ParseRequestURI(uri)
	if err != nil {
		return err
	}

	return nil
}

func (vw *ValidatingWebhook) validateSecret(secret *core.Secret) error {
	// Check secret type, it must be "kubernetes.io/dockerconfigjson"
	if secret.Type != core.SecretTypeDockerConfigJson {
		return fmt.Errorf("secret should be %s type", core.SecretTypeDockerConfigJson)
	}

	// Secret must contain ".dockerconfigjson" field
	dockerCfgRaw, ok := secret.Data[core.DockerConfigJsonKey]
	if !ok {
		return fmt.Errorf("secret should contain %s field", core.DockerConfigJsonKey)
	}

	// Check URI (scheme + address + path)
	scheme := string(secret.Data["scheme"])
	address := string(secret.Data["address"])
	path := string(secret.Data["path"])
	err := vw.checkURI(scheme, address, path)
	if err != nil {
		return err
	}

	dockerCfg := &DockerConfig{}
	err = json.Unmarshal(dockerCfgRaw, dockerCfg)
	if err != nil {
		return fmt.Errorf("can't umarshal docker config: %w", err)
	}

	if len(dockerCfg.Auths) == 0 {
		return fmt.Errorf("bad docker config")
	}

	// check registries in docker config
	for registry, authCfg := range dockerCfg.Auths {
		err = vw.registryClient.CheckImage(registry, vw.imageToCheck, authCfg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (vw *ValidatingWebhook) ValidatingWebhook(w http.ResponseWriter, r *http.Request) {
	// read request body
	var body []byte
	defer r.Body.Close()
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	logrus.Debug("AdmissionReview:")
	logrus.Debug(string(body))

	// Decode the request body into an admission review struct
	review := &admission.AdmissionReview{}
	err := json.Unmarshal(body, review)
	if err != nil {
		logrus.Errorf("can't unmarshal admission review: %v", err)
		http.Error(w, "can't unmarshal admission review", http.StatusBadRequest)

		return
	}

	if review.Request == nil {
		logrus.Errorf("bad admission review")
		http.Error(w, "bad admission review", http.StatusBadRequest)

		return
	}

	// Decode secret
	secretJSON := review.Request.Object.Raw

	secret := &core.Secret{}
	err = json.Unmarshal(secretJSON, secret)
	if err != nil {
		logrus.Errorf("can't unmarshal secret: %v", err)
		http.Error(w, "can't unmarshal secret", http.StatusBadRequest)

		return
	}

	// Respinse with same UID
	review.Response = &admission.AdmissionResponse{
		UID: review.Request.UID,
	}

	// Validate secret
	err = vw.validateSecret(secret)
	if err != nil {
		logrus.Errorf("validation of %s/%s secret failed: %v", secret.Namespace, secret.Name, err)
		review.Response.Allowed = false
		review.Response.Result = &meta.Status{
			Message: err.Error(),
		}
	} else {
		logrus.Infof("validation of the %s/%s secret was successful", secret.Namespace, secret.Name)
		review.Response.Allowed = true
	}

	// Send response
	reviewBytes, err := json.Marshal(review)
	if err != nil {
		logrus.Errorf("failed to marshal review: %v", err)
		http.Error(w, "failed to marshal review", http.StatusInternalServerError)

		return
	}
	_, _ = w.Write(reviewBytes)
}
