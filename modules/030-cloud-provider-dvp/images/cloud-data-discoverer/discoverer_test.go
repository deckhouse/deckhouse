/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("DVP Cloud discovery data tests", func() {

	var (
		fakeKubeClient *fake.Clientset
		d              Discoverer
	)

	BeforeEach(func() {
		logger := app.InitLogger()
		fakeKubeClient = fake.NewSimpleClientset()

		d = Discoverer{
			logger: logger,
			client: fakeKubeClient,
		}
	})

	Describe("Run", func() {

		Context("No storage classes in the cluster", func() {
			It("should return no error", func() {
				data, err := d.DiscoveryData(context.TODO(), []byte{})
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(MatchJSON(`{}`))
			})
		})

		Context("Two storage classes exist in the cluster", func() {
			It("should return no error", func() {

				_, err := fakeKubeClient.StorageV1().StorageClasses().Create(context.TODO(), &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "test-0"}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				_, err = fakeKubeClient.StorageV1().StorageClasses().Create(context.TODO(), &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "test-1"}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				data, err := d.DiscoveryData(context.TODO(), []byte{})
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(MatchJSON(`{"storageClasses":[{"name":"test-0"},{"name":"test-1"}]}`))
			})
		})

	})
})

func TestFencingAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FencingAgent Suite")
}
