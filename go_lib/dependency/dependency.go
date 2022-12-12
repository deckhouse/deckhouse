/*
Copyright 2021 Flant JSC

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

package dependency

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/kube-client/fake"
	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/dependency/etcd"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/dependency/vsphere"
)

// Container with external dependencies
type Container interface {
	GetHTTPClient(options ...http.Option) http.Client
	GetEtcdClient(endpoints []string, options ...etcd.Option) (etcd.Client, error)
	MustGetEtcdClient(endpoints []string, options ...etcd.Option) etcd.Client
	GetK8sClient(options ...k8s.Option) (k8s.Client, error)
	MustGetK8sClient(options ...k8s.Option) k8s.Client
	GetRegistryClient(repo string, options ...cr.Option) (cr.Client, error)
	GetVsphereClient(config *vsphere.ProviderClusterConfiguration) (vsphere.Client, error)
}

var (
	defaultDC Container
	TestDC    *mockedDependencyContainer
)

func init() {
	TestDC = newMockedContainer()
	defaultDC = NewDependencyContainer()
}

// NewDependencyContainer creates new Dependency container with external clients
func NewDependencyContainer() Container {
	return &dependencyContainer{}
}

type dependencyContainer struct {
	// etcdClient    etcd.Client
	k8sClient     k8s.Client
	crClient      cr.Client
	vsphereClient vsphere.Client

	m          sync.RWMutex
	isTestEnv  *bool
	httpClient http.Client
}

func (dc *dependencyContainer) isTestEnvironment() bool {
	dc.m.RLock()
	if dc.isTestEnv != nil {
		defer dc.m.RUnlock()
		return *dc.isTestEnv
	}
	dc.m.RUnlock()

	isTestEnvStr := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	isTestEnv, _ := strconv.ParseBool(isTestEnvStr)
	dc.m.Lock()
	dc.isTestEnv = &isTestEnv
	dc.m.Unlock()

	return *dc.isTestEnv
}

func (dc *dependencyContainer) GetHTTPClient(options ...http.Option) http.Client {
	if dc.isTestEnvironment() {
		return TestDC.GetHTTPClient(options...)
	}

	dc.m.RLock()
	if dc.httpClient != nil {
		defer dc.m.RUnlock()
		return dc.httpClient
	}
	dc.m.RUnlock()

	dc.m.Lock()
	defer dc.m.Unlock()

	var opts []http.Option
	opts = append(opts, options...)

	contentCA, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err == nil {
		opts = append(opts, http.WithAdditionalCACerts([][]byte{contentCA}))
	}

	dc.httpClient = http.NewClient(opts...)

	return dc.httpClient
}

func (dc *dependencyContainer) GetEtcdClient(endpoints []string, options ...etcd.Option) (etcd.Client, error) {
	if dc.isTestEnvironment() {
		return TestDC.GetEtcdClient(endpoints, options...)
	}

	cli, err := etcd.New(endpoints, options...)
	if err != nil {
		return nil, err
	}

	return cli, nil
}

func (dc *dependencyContainer) MustGetEtcdClient(endpoints []string, options ...etcd.Option) etcd.Client {
	client, err := dc.GetEtcdClient(endpoints, options...)
	if err != nil {
		panic(err)
	}
	return client
}

func (dc *dependencyContainer) GetK8sClient(options ...k8s.Option) (k8s.Client, error) {
	if dc.isTestEnvironment() {
		return TestDC.GetK8sClient(options...)
	}

	if dc.k8sClient == nil {
		kc, err := k8s.NewClient(options...)
		if err != nil {
			return nil, err
		}
		dc.k8sClient = kc
	}

	return dc.k8sClient, nil
}

func (dc *dependencyContainer) MustGetK8sClient(options ...k8s.Option) k8s.Client {
	client, err := dc.GetK8sClient(options...)
	if err != nil {
		panic(err)
	}
	return client
}

func (dc *dependencyContainer) GetRegistryClient(repo string, options ...cr.Option) (cr.Client, error) {
	if dc.isTestEnvironment() {
		return TestDC.GetRegistryClient(repo, options...)
	}

	// Maybe we should use multitone here
	// if dc.crClient != nil {
	// 	return dc.crClient, nil
	// }

	client, err := cr.NewClient(repo, options...)
	if err != nil {
		return nil, err
	}

	dc.crClient = client
	return client, nil
}

func (dc *dependencyContainer) GetVsphereClient(config *vsphere.ProviderClusterConfiguration) (vsphere.Client, error) {
	if dc.isTestEnvironment() {
		return TestDC.GetVsphereClient(config)
	}

	client, err := vsphere.NewClient(config)
	if err != nil {
		return nil, err
	}

	dc.vsphereClient = client
	return client, nil
}

// WithExternalDependencies decorate function with external dependencies
func WithExternalDependencies(f func(input *go_hook.HookInput, dc Container) error) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		return f(input, defaultDC)
	}
}

// Mocks
type mockedDependencyContainer struct {
	ctrl *minimock.Controller // maybe we need it somewhere in tests

	HTTPClient    *http.ClientMock
	EtcdClient    *etcd.ClientMock
	K8sClient     k8s.Client
	CRClient      *cr.ClientMock
	VsphereClient *vsphere.ClientMock
}

func (mdc *mockedDependencyContainer) GetHTTPClient(options ...http.Option) http.Client {
	return mdc.HTTPClient
}

func (mdc *mockedDependencyContainer) GetEtcdClient(endpoints []string, options ...etcd.Option) (etcd.Client, error) {
	return mdc.EtcdClient, nil
}

func (mdc *mockedDependencyContainer) MustGetEtcdClient(endpoints []string, options ...etcd.Option) etcd.Client {
	return mdc.EtcdClient
}

func (mdc *mockedDependencyContainer) GetK8sClient(options ...k8s.Option) (k8s.Client, error) {
	if mdc.K8sClient != nil {
		return mdc.K8sClient, nil
	}
	return fake.NewFakeCluster(k8s.DefaultFakeClusterVersion).Client, nil
}

func (mdc *mockedDependencyContainer) MustGetK8sClient(options ...k8s.Option) k8s.Client {
	k, _ := mdc.GetK8sClient(options...)
	return k
}

func (mdc *mockedDependencyContainer) GetRegistryClient(string, ...cr.Option) (cr.Client, error) {
	if mdc.CRClient != nil {
		return mdc.CRClient, nil
	}
	return nil, fmt.Errorf("no CR client")
}

func (mdc *mockedDependencyContainer) GetVsphereClient(config *vsphere.ProviderClusterConfiguration) (vsphere.Client, error) {
	if mdc.VsphereClient != nil {
		return mdc.VsphereClient, nil
	}
	return nil, fmt.Errorf("no Vsphere client")
}

// SetK8sVersion change FakeCluster versions. KubeClient returns with resources of specified version
func (mdc *mockedDependencyContainer) SetK8sVersion(ver k8s.FakeClusterVersion) {
	cli := fake.NewFakeCluster(ver).Client
	mdc.K8sClient = cli
}

func newMockedContainer() *mockedDependencyContainer {
	// ctrl := minimock.NewController(ginkgo.GinkgoT()) // gingo panics cause of offset
	ctrl := minimock.NewController(&testing.T{})
	return &mockedDependencyContainer{
		ctrl: ctrl,

		HTTPClient:    http.NewClientMock(ctrl),
		EtcdClient:    etcd.NewClientMock(ctrl),
		K8sClient:     fake.NewFakeCluster(k8s.DefaultFakeClusterVersion).Client,
		CRClient:      cr.NewClientMock(ctrl),
		VsphereClient: vsphere.NewClientMock(ctrl),
	}
}
