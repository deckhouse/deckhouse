/*
Copyright 2026 Flant JSC

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

package signature

import (
	"k8s.io/client-go/kubernetes"
)

type NoRenewer struct{}

func NewNoSignatureRenewer() Renewer {
	return &NoRenewer{}
}

func (s *NoRenewer) Renew(k8sInterface kubernetes.Interface) error {
	logger.Info("Skip renew signature. Not support in current edition")

	return nil
}

func (s *NoRenewer) APIServerChecksumPaths() []string {
	return nil
}
