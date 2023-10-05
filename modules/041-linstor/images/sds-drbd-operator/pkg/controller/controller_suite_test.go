package controller_test

import (
	"context"
	. "github.com/LINBIT/golinstor/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sds-drbd-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

func NewFakeClient() client.WithWatch {
	s := scheme.Scheme
	_ = metav1.AddMetaToScheme(s)
	_ = v1alpha1.AddToScheme(s)

	builder := fake.NewClientBuilder().WithScheme(s)

	cl := builder.Build()
	return cl
}

func NewLinstorClientWithMockNodes() (*Client, error) {
	lc, err := NewClient()
	lc.Nodes = MockNodes()

	return lc, err
}

func MockNodes() *NodeProviderMock {
	return &NodeProviderMock{}
}

type NodeProviderMock struct {
}

func (m *NodeProviderMock) GetAll(ctx context.Context, opts ...*ListOpts) ([]Node, error) {
	return nil, nil
}

func (m *NodeProviderMock) Get(ctx context.Context, nodeName string, opts ...*ListOpts) (Node, error) {
	return Node{}, nil
}

func (m *NodeProviderMock) Create(ctx context.Context, node Node) error {
	return nil
}

func (m *NodeProviderMock) CreateEbsNode(ctx context.Context, name string, remoteName string) error {
	return nil
}

func (m *NodeProviderMock) Modify(ctx context.Context, nodeName string, props NodeModify) error {
	return nil
}

func (m *NodeProviderMock) Delete(ctx context.Context, nodeName string) error {
	return nil
}

func (m *NodeProviderMock) Lost(ctx context.Context, nodeName string) error {
	return nil
}

func (m *NodeProviderMock) Reconnect(ctx context.Context, nodeName string) error {
	return nil
}

func (m *NodeProviderMock) GetNetInterfaces(ctx context.Context, nodeName string, opts ...*ListOpts) ([]NetInterface, error) {
	return nil, nil
}

func (m *NodeProviderMock) GetNetInterface(ctx context.Context, nodeName, nifName string, opts ...*ListOpts) (NetInterface, error) {
	return NetInterface{}, nil
}

func (m *NodeProviderMock) CreateNetInterface(ctx context.Context, nodeName string, nif NetInterface) error {
	return nil
}

func (m *NodeProviderMock) ModifyNetInterface(ctx context.Context, nodeName, nifName string, nif NetInterface) error {
	return nil
}

func (m *NodeProviderMock) DeleteNetinterface(ctx context.Context, nodeName, nifName string) error {
	return nil
}

func (m *NodeProviderMock) GetStoragePoolView(ctx context.Context, opts ...*ListOpts) ([]StoragePool, error) {
	return nil, nil
}
func (m *NodeProviderMock) GetStoragePools(ctx context.Context, nodeName string, opts ...*ListOpts) ([]StoragePool, error) {
	return nil, nil
}

func (m *NodeProviderMock) GetStoragePool(ctx context.Context, nodeName, spName string, opts ...*ListOpts) (StoragePool, error) {
	return StoragePool{}, nil
}
func (m *NodeProviderMock) CreateStoragePool(ctx context.Context, nodeName string, sp StoragePool) error {
	return nil
}
func (m *NodeProviderMock) ModifyStoragePool(ctx context.Context, nodeName, spName string, genericProps GenericPropsModify) error {
	return nil
}
func (m *NodeProviderMock) DeleteStoragePool(ctx context.Context, nodeName, spName string) error {
	return nil
}
func (m *NodeProviderMock) CreateDevicePool(ctx context.Context, nodeName string, psc PhysicalStorageCreate) error {
	return nil
}
func (m *NodeProviderMock) GetPhysicalStorageView(ctx context.Context, opts ...*ListOpts) ([]PhysicalStorageViewItem, error) {
	return nil, nil
}
func (m *NodeProviderMock) GetPhysicalStorage(ctx context.Context, nodeName string) ([]PhysicalStorageNode, error) {
	return nil, nil
}
func (m *NodeProviderMock) GetStoragePoolPropsInfos(ctx context.Context, nodeName string, opts ...*ListOpts) ([]PropsInfo, error) {
	return nil, nil
}
func (m *NodeProviderMock) GetPropsInfos(ctx context.Context, opts ...*ListOpts) ([]PropsInfo, error) {
	return nil, nil
}
func (m *NodeProviderMock) Evict(ctx context.Context, nodeName string) error {
	return nil
}
func (m *NodeProviderMock) Restore(ctx context.Context, nodeName string, restore NodeRestore) error {
	return nil
}
func (m *NodeProviderMock) Evacuate(ctx context.Context, nodeName string) error {
	return nil
}
