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
	kuberetry "k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func labelKey(name string) string {
	return fmt.Sprintf("dhctl.deckhouse.io/%s", name)
}

type StateCache struct {
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
	_, err := c.populateSecret()
	return err
}

func (c *StateCache) WithLabels(labels map[string]string) *StateCache {
	c.labels = labels
	return c
}

func (c *StateCache) getSecret() (*v1.Secret, error) {
	s, err := c.populateSecret()
	if err != nil {
		return nil, err
	}

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}

	return s, nil
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

func (c *StateCache) update(action func(map[string][]byte) map[string][]byte) error {
	return kuberetry.RetryOnConflict(kuberetry.DefaultBackoff, func() error {
		s, err := c.getSecret()
		if err != nil {
			return err
		}

		s.Data = action(s.Data)

		_, err = c.secretsAPI.Update(context.TODO(), s, metav1.UpdateOptions{})

		return err
	})
}

func (c *StateCache) get(s *v1.Secret, key string) ([]byte, error) {
	data := s.Data[key]
	decodedData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		log.ErrorF("Cannot decode cache %s val %v\n", key, err)
		return nil, err
	}

	return decodedData, nil
}

func (c *StateCache) prepareContent(content []byte) []byte {
	// todo remove encoding
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(content)))
	base64.StdEncoding.Encode(buf, content)

	return buf
}

func (c *StateCache) Save(name string, content []byte) error {
	buf := c.prepareContent(content)

	return c.update(func(curState map[string][]byte) map[string][]byte {
		curState[name] = buf
		return curState
	})
}

func (c *StateCache) SaveStruct(name string, v interface{}) error {
	b := new(bytes.Buffer)
	err := gob.NewEncoder(b).Encode(v)
	if err != nil {
		return err
	}

	return c.Save(name, b.Bytes())
}

func (c *StateCache) Load(name string) ([]byte, error) {
	s, err := c.getSecret()
	if err != nil {
		log.ErrorF("Cannot get secret %s val %v\n", name, err)
		return nil, err
	}

	return c.get(s, name)
}

func (c *StateCache) LoadStruct(name string, v interface{}) error {
	d, err := c.Load(name)
	if err != nil {
		return err
	}

	return gob.NewDecoder(bytes.NewBuffer(d)).Decode(v)
}

func (c *StateCache) Delete(name string) {
	err := c.update(func(curState map[string][]byte) map[string][]byte {
		delete(curState, name)

		return curState
	})
	if err != nil {
		log.ErrorF("Cannot delete cache %s val %v\n", name, err)
	}
}

func (c *StateCache) CleanWithExceptions(excludeKeys ...string) {
	err := c.update(func(curState map[string][]byte) map[string][]byte {
		newState := map[string][]byte{
			state.TombstoneKey: c.prepareContent([]byte("yes")),
		}

		for _, k := range excludeKeys {
			v, ok := curState[k]
			if !ok {
				continue
			}

			newState[k] = v
		}

		return newState
	})
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
	s, err := c.getSecret()
	if err != nil {
		return err
	}

	data := s.Data

	if len(data) == 0 {
		return nil
	}

	keys := make([]string, 0)
	for name := range data {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	for _, k := range keys {
		d, err := c.get(s, k)
		if err != nil {
			return err
		}

		err = action(k, d)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *StateCache) InCache(name string) (bool, error) {
	s, err := c.getSecret()
	if err != nil {
		return false, err
	}
	_, ok := s.Data[name]

	return ok, nil
}

func (c *StateCache) NeedIntermediateSave() bool {
	// cache store in k8s secret, need sync every intermediate states
	return true
}

func (c *StateCache) Dir() string {
	return c.tmpDir
}
