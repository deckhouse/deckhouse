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

package internal

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: internal :: storage classes ::", func() {
	DescribeTable("GetStorageClassName",
		func(input, expected string) {
			Expect(GetStorageClassName(input)).To(Equal(expected))
		},
		Entry("keeps valid name", "replicated", "replicated"),
		Entry("normalizes spaces and case", "Excluded Fast", "excluded-fast"),
		Entry("removes invalid symbols and trims ends", "-Xx__$()? -foo-", "xx--foo"),
		Entry("trims dots and dashes", ".. YY fast SSD-foo.-", "yy-fast-ssd-foo"),
	)

	DescribeTable("NewExcludes",
		func(patterns []string, name string, expected bool) {
			excludes, err := NewExcludes(patterns)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(excludes.Match(name)).To(Equal(expected))
		},
		Entry("matches nothing when there are no patterns", nil, "fast", false),
		Entry("matches an exact name", []string{"fast"}, "fast", true),
		Entry("does not match a name containing the pattern", []string{"fast"}, "ultra-fast-ssd", false),
		Entry("does not match a name the pattern starts with", []string{"fast"}, "fast-ssd", false),
		Entry("matches the whole name with a wildcard pattern", []string{"excluded-.*"}, "excluded-legacy", true),
		Entry("does not match a prefix with a wildcard pattern", []string{".*-fast"}, "ultra-fast-ssd", false),
		Entry("keeps explicit anchors working", []string{"^fast$"}, "fast", true),
		Entry("anchors alternations as a whole", []string{"fast|slow"}, "slow", true),
		Entry("does not match a part of an alternation", []string{"fast|slow"}, "very-slow", false),
		Entry("matches when any pattern matches", []string{"fast", "slow"}, "slow", true),
	)

	It("NewExcludes fails on an invalid pattern", func() {
		_, err := NewExcludes([]string{"excluded-([a-z"})
		Expect(err).Should(HaveOccurred())
	})

	DescribeTable("StorageClassToValue",
		func(input *storagev1.StorageClass, expected StorageClass) {
			Expect(StorageClassToValue(input)).To(Equal(expected))
		},
		Entry("uses defaults for nil optional fields",
			&storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "replicated",
				},
				Parameters: map[string]string{
					"dvpStorageClass": "replicated",
				},
			},
			StorageClass{
				Name:                 "replicated",
				DVPStorageClass:      "replicated",
				VolumeBindingMode:    string(DefaultVolumeBindingMode),
				ReclaimPolicy:        string(corev1.PersistentVolumeReclaimDelete),
				AllowVolumeExpansion: false,
				IsDefault:            false,
			},
		),
		Entry("reads stable default annotation and optional fields",
			&storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "stable-default",
					Annotations: map[string]string{
						StableDefaultAnnotation: "TrUe",
					},
				},
				Parameters: map[string]string{
					"dvpStorageClass": "stable-default",
				},
				ReclaimPolicy:        ptrTo(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: ptrTo(true),
			},
			StorageClass{
				Name:                 "stable-default",
				DVPStorageClass:      "stable-default",
				VolumeBindingMode:    string(DefaultVolumeBindingMode),
				ReclaimPolicy:        string(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: true,
				IsDefault:            true,
			},
		),
		Entry("reads beta default annotation",
			&storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "beta-default",
					Annotations: map[string]string{
						BetaDefaultAnnotation: "true",
					},
				},
				Parameters: map[string]string{
					"dvpStorageClass": "beta-default",
				},
			},
			StorageClass{
				Name:                 "beta-default",
				DVPStorageClass:      "beta-default",
				VolumeBindingMode:    string(DefaultVolumeBindingMode),
				ReclaimPolicy:        string(corev1.PersistentVolumeReclaimDelete),
				AllowVolumeExpansion: false,
				IsDefault:            true,
			},
		),
	)
})

func ptrTo[T any](v T) *T {
	return &v
}
