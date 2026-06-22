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

package registry

import (
	"testing"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

func fakePKI() PKI {
	return PKI{
		CA:     PKICertKey{Cert: "CA", Key: "K"},
		ROUser: PKIUser{Name: "ro", Password: "rop", PasswordHash: "roh"},
		RWUser: PKIUser{Name: "rw", Password: "rwp", PasswordHash: "rwh"},
	}
}

func TestCleanModelAirGap(t *testing.T) {
	mc := module_config.RegistryModuleConfig{
		Settings: module_config.CleanSettings{Cache: module_config.CacheSettings{Enabled: true, StorageSize: "20Gi"}},
	}
	m, err := NewCleanModel(mc, "")
	if err != nil {
		t.Fatal(err)
	}
	if !m.NeedsSeed() {
		t.Fatal("air-gap must need seed")
	}
	ctx, err := m.BashibleContext(func() (PKI, error) { return fakePKI(), nil })
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.RegistryModuleEnable || ctx.Bootstrap == nil || !ctx.Bootstrap.Seed {
		t.Fatalf("air-gap ctx must enable module + bootstrap.seed: %+v", ctx)
	}
	if _, ok := ctx.Hosts["registry.d8-system.svc:5001"]; !ok {
		t.Fatalf("air-gap ctx must have primary host: %+v", ctx.Hosts)
	}
	if m.RemoteData().ImagesRepo != constant.BundleImagesRepo {
		t.Fatalf("air-gap RemoteData must be bundle, got %q", m.RemoteData().ImagesRepo)
	}
}

func TestCleanModelDirect(t *testing.T) {
	mc := module_config.RegistryModuleConfig{
		Settings: module_config.CleanSettings{
			Cache:    module_config.CacheSettings{Enabled: false},
			Upstream: &module_config.UpstreamSettings{Host: "registry.example.com", Path: "/deckhouse/ee", Scheme: constant.SchemeHTTPS},
		},
	}
	m, err := NewCleanModel(mc, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.NeedsSeed() {
		t.Fatal("direct must not need seed")
	}
	ctx, err := m.BashibleContext(func() (PKI, error) { return fakePKI(), nil })
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Bootstrap != nil && ctx.Bootstrap.Seed {
		t.Fatal("direct must not set bootstrap.seed")
	}
	if m.RemoteData().ImagesRepo != "registry.example.com/deckhouse/ee" {
		t.Fatalf("direct RemoteData wrong: %q", m.RemoteData().ImagesRepo)
	}
}

func TestCleanModelUnmanaged(t *testing.T) {
	f := false
	mc := module_config.RegistryModuleConfig{Enabled: &f}
	m, err := NewCleanModel(mc, "ext.example.com/path")
	if err != nil {
		t.Fatal(err)
	}
	if m.Managed {
		t.Fatal("enabled:false must be unmanaged")
	}
	ctx, err := m.BashibleContext(func() (PKI, error) { return fakePKI(), nil })
	if err != nil {
		t.Fatal(err)
	}
	if ctx.RegistryModuleEnable {
		t.Fatal("unmanaged must not enable module")
	}
	if m.KubeadmContext().Address == "" {
		t.Fatal("unmanaged must derive kubeadm address from initImagesRepo")
	}
}

func TestCleanModelConnectedWithCache(t *testing.T) {
	mc := module_config.RegistryModuleConfig{
		Settings: module_config.CleanSettings{
			Cache:    module_config.CacheSettings{Enabled: true, StorageSize: "20Gi"},
			Upstream: &module_config.UpstreamSettings{Host: "registry.example.com", Path: "/deckhouse/ee", Scheme: constant.SchemeHTTPS},
		},
	}
	m, err := NewCleanModel(mc, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.NeedsSeed() {
		t.Fatal("connected+cache must not need seed (upstream present)")
	}
	ctx, err := m.BashibleContext(func() (PKI, error) { return fakePKI(), nil })
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Bootstrap == nil || ctx.Bootstrap.Seed {
		t.Fatalf("connected+cache bootstrap.seed must be false: %+v", ctx.Bootstrap)
	}
	if m.RemoteData().ImagesRepo != "registry.example.com/deckhouse/ee" {
		t.Fatalf("connected+cache RemoteData wrong: %q", m.RemoteData().ImagesRepo)
	}
}
