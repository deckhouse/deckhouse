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
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// nolint: govet
func Test_ExampleMockedHTTPClient(_ *testing.T) {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")

	dependency.TestDC.HTTPClient.DoMock.Expect(&http.Request{}).Return(&http.Response{
		StatusCode: http.StatusTeapot,
		Status:     "Im teapot",
	}, nil)

	_ = dependency.WithExternalDependencies(someHandlerWithHTTP)(nil, nil)
	// Output: 418

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

// nolint: govet
func Test_ExampleK8sClient(_ *testing.T) {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	dependency.TestDC.SetK8sVersion(k8s.V117)
	_, _ = dependency.TestDC.K8sClient.CoreV1().Namespaces().Create(context.TODO(),
		&corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "default"}},
		v1.CreateOptions{},
	)

	_ = dependency.WithExternalDependencies(handlerWithK8S)(nil, nil)
	// Output: default

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

// nolint: govet
func Test_ExampleVersionedK8sClient(_ *testing.T) {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")

	dependency.TestDC.SetK8sVersion(k8s.V116)
	_ = dependency.WithExternalDependencies(handlerWithVersionedK8S)(nil, nil)

	dependency.TestDC.SetK8sVersion(k8s.V120)
	_ = dependency.WithExternalDependencies(handlerWithVersionedK8S)(nil, nil)

	// Output:
	// 32
	// 38

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

// nolint: govet
func Test_ExampleEtcdClient(_ *testing.T) {
	prev := os.Getenv("D8_IS_TESTS_ENVIRONMENT")
	os.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	dependency.TestDC.EtcdClient.GetMock.
		Expect(context.TODO(), "foo").
		Return(&clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{{Key: []byte("foo"), Value: []byte("bar")}}}, nil)

	_ = dependency.WithExternalDependencies(handlerWithEtcd)(nil, nil)
	// Output: bar

	os.Setenv("D8_IS_TESTS_ENVIRONMENT", prev)
}

func Test_Race(_ *testing.T) {
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

func someHandlerWithHTTP(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	ht := dc.GetHTTPClient()

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, _ := ht.Do(req)
	fmt.Println(resp.StatusCode)

	return nil
}

func handlerWithK8S(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	k8 := dc.MustGetK8sClient()
	ns, _ := k8.CoreV1().Namespaces().Get(context.TODO(), "default", v1.GetOptions{})
	fmt.Println(ns.Name)

	return nil
}

func handlerWithVersionedK8S(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	k8 := dc.MustGetK8sClient()
	_, res, _ := k8.Discovery().ServerGroupsAndResources()
	fmt.Println(len(res))

	return nil
}

func handlerWithEtcd(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	cl := dc.MustGetEtcdClient([]string{"http://192.168.1.104:2379"})
	v, _ := cl.Get(context.TODO(), "foo")
	fmt.Println(string(v.Kvs[0].Value))

	return nil
}
