// Copyright 2021 Flant CJSC
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

package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func labelKey(name string) string {
	return fmt.Sprintf("dhctl.deckhouse.io/%s", name)
}

type StateCache struct {
	secret     *v1.Secret
	secretsAPI typedv1core.SecretInterface

	labels map[string]string

	secretName string
	namespace  string
	tmpDir     string
}

func NewK8sStateCache(client *KubernetesClient, namespace, secretName, tmpDir string) *StateCache {
	secretsAPI := client.CoreV1().Secrets(namespace)
	return &StateCache{
		secretsAPI: secretsAPI,
		tmpDir:     tmpDir,
		labels:     make(map[string]string),
		secretName: secretName,
	}
}

func (c *StateCache) Init() error {
	secret, err := c.populateSecret()
	if err != nil {
		return err
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}

	c.secret = secret
	return nil
}

func (c *StateCache) WithLabels(labels map[string]string) *StateCache {
	c.labels = labels
	return c
}

func (c *StateCache) populateSecret() (*v1.Secret, error) {
	var secret *v1.Secret
	var lastError error
	err := retry.NewSilentLoop("get cache secret", 3, 2*time.Second).Run(func() error {
		var err error
		secret, err = c.secretsAPI.Get(context.TODO(), c.secretName, metav1.GetOptions{})
		if err == nil {
			return nil
		}

		if apierrors.IsNotFound(err) {
			secret = nil
			return nil
		}

		secret = nil
		lastError = err
		return err
	})
	if err != nil {
		return nil, lastError
	}

	if secret != nil {
		return secret, nil
	}

	preparedLabels := map[string]string{
		"heritage": "dhctl-job",

		labelKey("state"):        "true",
		labelKey("cluster-name"): c.secretName,
	}

	for k, v := range c.labels {
		preparedLabels[k] = v
	}

	err = retry.NewSilentLoop("save cache secret", 3, 2*time.Second).Run(func() error {
		var err error

		secretToCreate := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.secretName,
				Namespace: c.namespace,
				Labels:    preparedLabels,
			},

			Data: map[string][]byte{},
		}

		secret, err = c.secretsAPI.Create(context.TODO(), secretToCreate, metav1.CreateOptions{})
		if err == nil || apierrors.IsAlreadyExists(err) {
			return nil
		}

		lastError = err
		return err
	})

	if err != nil {
		return nil, lastError
	}

	return secret, nil
}

func (c *StateCache) update(secretToUpdate *v1.Secret) error {
	var lastErr error
	err := retry.NewSilentLoop("save cache secret", 3, 2*time.Second).Run(func() error {
		updatedSecret, err := c.secretsAPI.Update(context.TODO(), secretToUpdate, metav1.UpdateOptions{})
		if err == nil {
			c.secret = updatedSecret
			return nil
		}

		lastErr = err
		return err
	})
	if err != nil {
		return lastErr
	}

	return nil
}

func (c *StateCache) Save(name string, content []byte) error {
	encContent := []byte(base64.StdEncoding.EncodeToString(content))

	secretToUpdate := c.secret.DeepCopy()
	secretToUpdate.Data[name] = encContent

	return c.update(secretToUpdate)
}

func (c *StateCache) SaveStruct(name string, v interface{}) error {
	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(v)
	if err != nil {
		return err
	}

	return c.Save(name, b.Bytes())
}

func (c *StateCache) Load(name string) []byte {
	data, ok := c.secret.Data[name]
	if !ok {
		return nil
	}

	decodedData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		log.ErrorF("Cannot decode cache %s val %v\n", name, err)
		return nil
	}

	return decodedData
}

func (c *StateCache) LoadStruct(name string, v interface{}) error {
	d := c.Load(name)
	if d == nil {
		return fmt.Errorf("can't load struct")
	}

	return gob.NewDecoder(bytes.NewBuffer(d)).Decode(v)
}

func (c *StateCache) Delete(name string) {
	secretToUpdate := c.secret.DeepCopy()
	delete(secretToUpdate.Data, name)

	err := c.update(secretToUpdate)
	if err != nil {
		log.ErrorF("Cannot delete cache %s val %v\n", name, err)
	}
}

func (c *StateCache) CleanWithExceptions(excludeKeys ...string) {
	secretToUpdate := c.secret.DeepCopy()
	newState := map[string][]byte{
		state.TombstoneKey: []byte("yes"),
	}

	for _, k := range excludeKeys {
		v, ok := secretToUpdate.Data[k]
		if !ok {
			continue
		}

		newState[k] = v
	}

	secretToUpdate.Data = newState

	err := c.update(secretToUpdate)
	if err != nil {
		log.ErrorF("Cannot clean cache %v\n", err)
	}
}

func (c *StateCache) Clean() {
	c.CleanWithExceptions()
}

func (c *StateCache) GetPath(name string) string {
	return filepath.Join(c.tmpDir, name)
}

func (c *StateCache) Iterate(action func(string, []byte) error) error {
	if len(c.secret.Data) == 0 {
		return nil
	}

	keys := make([]string, 0)
	for name := range c.secret.Data {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	for _, k := range keys {
		err := action(k, c.secret.Data[k])
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *StateCache) InCache(name string) bool {
	_, ok := c.secret.Data[name]
	return ok
}
