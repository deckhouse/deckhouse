package dependency_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// nolint: govet
func ExampleMockedHTTPClient() {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")

	dependency.TestDC.HTTPClient.DoMock.Expect(&http.Request{}).Return(&http.Response{
		StatusCode: http.StatusTeapot,
		Status:     "Im teapot",
	}, nil)

	_ = dependency.WithExternalDependencies(someHandlerWithHTTP)(nil)
	// Output: 418

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

// nolint: govet
func ExampleK8sClient() {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	_, _ = dependency.TestDC.K8sClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "default"}})

	_ = dependency.WithExternalDependencies(handlerWithK8S)(nil)
	// Output: default

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

// nolint: govet
func ExampleEtcdClient() {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	dependency.TestDC.EtcdClient.GetMock.
		Expect(context.TODO(), "foo").
		Return(&clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{{Key: []byte("foo"), Value: []byte("bar")}}}, nil)

	_ = dependency.WithExternalDependencies(handlerWithEtcd)(nil)
	// Output: bar

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

func TestRace(t *testing.T) {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	dc := dependency.NewDependencyContainer()
	go func() {
		_, _ = dc.GetK8sClient()
	}()
	go func() {
		_ = dc.GetHTTPClient()
	}()
	go func() {
		_, _ = dc.GetEtcdClient([]string{})
	}()
	time.Sleep(100 * time.Millisecond)
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

func someHandlerWithHTTP(_ *go_hook.HookInput, dc dependency.Container) error {
	ht := dc.GetHTTPClient()

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, _ := ht.Do(req)
	fmt.Println(resp.StatusCode)

	return nil
}

func handlerWithK8S(_ *go_hook.HookInput, dc dependency.Container) error {
	k8 := dc.MustGetK8sClient()
	ns, _ := k8.CoreV1().Namespaces().Get("default", v1.GetOptions{})
	fmt.Println(ns.Name)

	return nil
}

func handlerWithEtcd(_ *go_hook.HookInput, dc dependency.Container) error {
	cl := dc.MustGetEtcdClient([]string{"http://192.168.1.104:2379"})
	v, _ := cl.Get(context.TODO(), "foo")
	fmt.Println(string(v.Kvs[0].Value))

	return nil
}
