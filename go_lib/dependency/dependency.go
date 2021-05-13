package dependency

import (
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/shell-operator/pkg/kube/fake"
	"github.com/gojuno/minimock/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/etcd"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// Container with external dependencies
type Container interface {
	GetHTTPClient(options ...http.Option) http.Client
	GetEtcdClient(endpoints []string, options ...etcd.Option) (etcd.Client, error)
	MustGetEtcdClient(endpoints []string, options ...etcd.Option) etcd.Client
	GetK8sClient(options ...k8s.Option) (k8s.Client, error)
	MustGetK8sClient(options ...k8s.Option) k8s.Client
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
	httpClient http.Client
	etcdClient etcd.Client
	k8sClient  k8s.Client

	m         sync.RWMutex
	isTestEnv *bool
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

	if dc.httpClient == nil {
		dc.httpClient = http.NewClient(options...)
	}

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

// WithExternalDependencies decorate function with external dependencies
func WithExternalDependencies(f func(input *go_hook.HookInput, dc Container) error) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		return f(input, defaultDC)
	}
}

// Mocks
type mockedDependencyContainer struct {
	ctrl *minimock.Controller // maybe we need it somewhere in tests

	HTTPClient *http.ClientMock
	EtcdClient *etcd.ClientMock
	K8sClient  k8s.Client
}

func (mdc mockedDependencyContainer) GetHTTPClient(options ...http.Option) http.Client {
	return mdc.HTTPClient
}

func (mdc mockedDependencyContainer) GetEtcdClient(endpoints []string, options ...etcd.Option) (etcd.Client, error) {
	return mdc.EtcdClient, nil
}

func (mdc mockedDependencyContainer) MustGetEtcdClient(endpoints []string, options ...etcd.Option) etcd.Client {
	return mdc.EtcdClient
}

func (mdc mockedDependencyContainer) GetK8sClient(options ...k8s.Option) (k8s.Client, error) {
	if mdc.K8sClient != nil {
		return mdc.K8sClient, nil
	}
	return fake.NewFakeCluster(k8s.DefaultFakeClusterVersion).KubeClient, nil
}

func (mdc mockedDependencyContainer) MustGetK8sClient(options ...k8s.Option) k8s.Client {
	k, _ := mdc.GetK8sClient(options...)
	return k
}

// SetK8sVersion change FakeCluster versions. KubeClient returns with resources of specified version
func (mdc *mockedDependencyContainer) SetK8sVersion(ver k8s.FakeClusterVersion) {
	cli := fake.NewFakeCluster(ver).KubeClient
	mdc.K8sClient = cli
}

func newMockedContainer() *mockedDependencyContainer {
	// ctrl := minimock.NewController(ginkgo.GinkgoT()) // gingo panics cause of offset
	ctrl := minimock.NewController(&testing.T{})
	return &mockedDependencyContainer{
		ctrl: ctrl,

		HTTPClient: http.NewClientMock(ctrl),
		EtcdClient: etcd.NewClientMock(ctrl),
		K8sClient:  fake.NewFakeCluster(k8s.DefaultFakeClusterVersion).KubeClient,
	}
}
