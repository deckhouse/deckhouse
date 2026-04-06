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

package template

import (
	"os"
	"path/filepath"
	"testing"
)

// allPKIFiles — полный набор файлов, которые CreatePKIBundle создаёт по умолчанию.
var allPKIFiles = []string{
	"ca.crt", "ca.key",
	"apiserver.crt", "apiserver.key",
	"apiserver-kubelet-client.crt", "apiserver-kubelet-client.key",
	"front-proxy-ca.crt", "front-proxy-ca.key",
	"front-proxy-client.crt", "front-proxy-client.key",
	"etcd/ca.crt", "etcd/ca.key",
	"etcd/server.crt", "etcd/server.key",
	"etcd/peer.crt", "etcd/peer.key",
	"etcd/healthcheck-client.crt", "etcd/healthcheck-client.key",
	"apiserver-etcd-client.crt", "apiserver-etcd-client.key",
	"sa.key", "sa.pub",
}

// makeTemplateData возвращает минимальный templateData, необходимый для PreparePKI.
func makeTemplateData(clusterDomain, serviceSubnetCIDR string) map[string]interface{} {
	return map[string]interface{}{
		"clusterConfiguration": map[string]interface{}{
			"clusterDomain":     clusterDomain,
			"serviceSubnetCIDR": serviceSubnetCIDR,
		},
	}
}

func TestPreparePKI_CreatesAllFiles(t *testing.T) {
	pkiDir := t.TempDir()

	templateData := makeTemplateData("cluster.local", "10.96.0.0/12")

	// PreparePKI жёстко прописывает "/tmp/" как pkiDir через pki.WithPKIDir.
	// Чтобы тест был изолирован и не загрязнял /tmp, мы временно подменяем
	// вызов через обёртку preparePKIWithDir, которую добавим ниже.
	// Пока тестируем публичный API напрямую, передавая pkiDir через замену.
	err := preparePKIWithDir(nil, "master-0", "10.0.0.1", templateData, pkiDir)
	if err != nil {
		t.Fatalf("PreparePKI вернул ошибку: %v", err)
	}

	for _, f := range allPKIFiles {
		path := filepath.Join(pkiDir, "pki", f)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("ожидался файл %q, но он не найден: %v", f, statErr)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("файл %q пустой", f)
		}
	}
}

func TestPreparePKI_Idempotent(t *testing.T) {
	pkiDir := t.TempDir()
	templateData := makeTemplateData("cluster.local", "10.96.0.0/12")

	// Первый вызов
	if err := preparePKIWithDir(nil, "master-0", "10.0.0.1", templateData, pkiDir); err != nil {
		t.Fatalf("первый вызов PreparePKI вернул ошибку: %v", err)
	}

	// Читаем содержимое файлов после первого вызова
	before := readPKIFiles(t, pkiDir)

	// Второй вызов — должен быть идемпотентным (CA не перегенерируются)
	if err := preparePKIWithDir(nil, "master-0", "10.0.0.1", templateData, pkiDir); err != nil {
		t.Fatalf("второй вызов PreparePKI вернул ошибку: %v", err)
	}

	after := readPKIFiles(t, pkiDir)

	for _, f := range []string{"ca.crt", "ca.key", "etcd/ca.crt", "etcd/ca.key", "front-proxy-ca.crt", "front-proxy-ca.key"} {
		if string(before[f]) != string(after[f]) {
			t.Errorf("CA файл %q изменился после повторного вызова (нарушена идемпотентность)", f)
		}
	}
}

func TestPreparePKI_DifferentServiceCIDR(t *testing.T) {
	tests := []struct {
		name          string
		clusterDomain string
		serviceCIDR   string
	}{
		{
			name:          "стандартный CIDR",
			clusterDomain: "cluster.local",
			serviceCIDR:   "10.96.0.0/12",
		},
		{
			name:          "нестандартный CIDR",
			clusterDomain: "mycluster.internal",
			serviceCIDR:   "192.168.0.0/16",
		},
		{
			name:          "маленький CIDR /24",
			clusterDomain: "k8s.local",
			serviceCIDR:   "172.20.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkiDir := t.TempDir()
			templateData := makeTemplateData(tt.clusterDomain, tt.serviceCIDR)

			err := preparePKIWithDir(nil, "master-0", "10.0.0.1", templateData, pkiDir)
			if err != nil {
				t.Fatalf("PreparePKI(%q, %q) вернул ошибку: %v", tt.clusterDomain, tt.serviceCIDR, err)
			}

			// Проверяем, что хотя бы основной CA создан
			caPath := filepath.Join(pkiDir, "pki", "ca.crt")
			if _, err := os.Stat(caPath); err != nil {
				t.Errorf("ca.crt не создан: %v", err)
			}
		})
	}
}

func TestPreparePKI_MissingClusterDomain(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("ожидалась паника при отсутствии clusterDomain, но её не было")
		}
	}()

	// templateData без clusterDomain — должна быть паника при type assertion
	templateData := map[string]interface{}{
		"clusterConfiguration": map[string]interface{}{
			"serviceSubnetCIDR": "10.96.0.0/12",
		},
	}

	_ = preparePKIWithDir(nil, "master-0", "10.0.0.1", templateData, t.TempDir())
}

func TestPreparePKI_MissingServiceSubnetCIDR(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("ожидалась паника при отсутствии serviceSubnetCIDR, но её не было")
		}
	}()

	// templateData без serviceSubnetCIDR — должна быть паника при type assertion
	templateData := map[string]interface{}{
		"clusterConfiguration": map[string]interface{}{
			"clusterDomain": "cluster.local",
		},
	}

	_ = preparePKIWithDir(nil, "master-0", "10.0.0.1", templateData, t.TempDir())
}

// readPKIFiles читает содержимое всех PKI файлов из директории.
func readPKIFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte, len(allPKIFiles))
	for _, f := range allPKIFiles {
		data, err := os.ReadFile(filepath.Join(dir, "pki", f))
		if err != nil {
			t.Fatalf("не удалось прочитать файл %q: %v", f, err)
		}
		result[f] = data
	}
	return result
}
