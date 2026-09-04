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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	clouddatav1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

var _ = Describe("MergeDiscoveryData", func() {
	fullDiscoveryData := func() clouddatav1.YandexCloudDiscoveryData {
		return clouddatav1.YandexCloudDiscoveryData{
			RouteTableID:                  "rt-new",
			DefaultLbTargetGroupNetworkID: "net-new",
			InternalNetworkIDs:            []string{"net-new"},
			Zones:                         []string{"ru-central1-a"},
			ZoneToSubnetIDMap:             map[string]string{"ru-central1-a": "subnet-new"},
			ShouldAssignPublicIPAddress:   ptr.To(true),
			NATInstanceName:               "nat-new",
			NATInstanceZone:               "ru-central1-a",
			MonitoringAPIKey:              "key-new",
		}
	}

	It("stamps the type markers and the region even on an empty pair", func() {
		result := MergeDiscoveryData(clouddatav1.YandexCloudDiscoveryData{}, clouddatav1.YandexCloudDiscoveryData{})

		Expect(result.APIVersion).To(Equal(clouddatav1.APIVersion))
		Expect(result.Kind).To(Equal(clouddatav1.YandexCloudDiscoveryDataKind))
		Expect(result.Region).To(Equal(clouddatav1.YandexCloudDiscoveryDataDefaultRegion))
	})

	It("grafts every field onto an empty current value", func() {
		result := MergeDiscoveryData(fullDiscoveryData(), clouddatav1.YandexCloudDiscoveryData{})

		Expect(result.RouteTableID).To(Equal("rt-new"))
		Expect(result.DefaultLbTargetGroupNetworkID).To(Equal("net-new"))
		Expect(result.InternalNetworkIDs).To(Equal([]string{"net-new"}))
		Expect(result.Zones).To(Equal([]string{"ru-central1-a"}))
		Expect(result.ZoneToSubnetIDMap).To(Equal(map[string]string{"ru-central1-a": "subnet-new"}))
		Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeTrue()))
		Expect(result.NATInstanceName).To(Equal("nat-new"))
		Expect(result.NATInstanceZone).To(Equal("ru-central1-a"))
		Expect(result.MonitoringAPIKey).To(Equal("key-new"))
	})

	It("keeps every already-populated field of the current value", func() {
		current := clouddatav1.YandexCloudDiscoveryData{
			RouteTableID:                  "rt-old",
			DefaultLbTargetGroupNetworkID: "net-old",
			InternalNetworkIDs:            []string{"net-old"},
			Zones:                         []string{"ru-central1-b"},
			ZoneToSubnetIDMap:             map[string]string{"ru-central1-b": "subnet-old"},
			NATInstanceName:               "nat-old",
			NATInstanceZone:               "ru-central1-b",
			MonitoringAPIKey:              "key-old",
		}

		result := MergeDiscoveryData(fullDiscoveryData(), current)

		Expect(result.RouteTableID).To(Equal("rt-old"))
		Expect(result.DefaultLbTargetGroupNetworkID).To(Equal("net-old"))
		Expect(result.InternalNetworkIDs).To(Equal([]string{"net-old"}))
		Expect(result.Zones).To(Equal([]string{"ru-central1-b"}))
		Expect(result.ZoneToSubnetIDMap).To(Equal(map[string]string{"ru-central1-b": "subnet-old"}))
		Expect(result.NATInstanceName).To(Equal("nat-old"))
		Expect(result.NATInstanceZone).To(Equal("ru-central1-b"))
		Expect(result.MonitoringAPIKey).To(Equal("key-old"))
	})

	// shouldAssignPublicIPAddress is a pointer precisely so that "not discovered yet" and
	// "discovered as false" stay distinguishable. With a plain bool the WithoutNAT-only
	// `true` could never be turned back off, and a discovered `false` was indistinguishable
	// from an absent value — which is what put a bare `false` into values for a cluster that
	// had no discovery data at all.
	Describe("shouldAssignPublicIPAddress", func() {
		It("takes a discovered false when the current value is unset", func() {
			result := MergeDiscoveryData(
				clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(false)},
				clouddatav1.YandexCloudDiscoveryData{},
			)

			Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeFalse()))
		})

		It("takes a discovered true when the current value is unset", func() {
			result := MergeDiscoveryData(
				clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(true)},
				clouddatav1.YandexCloudDiscoveryData{},
			)

			Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeTrue()))
		})

		It("does not overwrite a current false with a new true", func() {
			result := MergeDiscoveryData(
				clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(true)},
				clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(false)},
			)

			Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeFalse()))
		})

		It("stays unset when neither side carries it", func() {
			result := MergeDiscoveryData(clouddatav1.YandexCloudDiscoveryData{}, clouddatav1.YandexCloudDiscoveryData{})

			Expect(result.ShouldAssignPublicIPAddress).To(BeNil())
		})

		It("does not alias the pointer of the new value", func() {
			newValue := clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(true)}

			result := MergeDiscoveryData(newValue, clouddatav1.YandexCloudDiscoveryData{})
			*newValue.ShouldAssignPublicIPAddress = false

			Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeTrue()))
		})
	})

	// The hook writes the merged struct straight into
	// cloudProviderYandex.internal.providerDiscoveryData, and that path is validated against
	// openapi/values.yaml on every write. A cluster whose infrastructure DKP does not create
	// has nothing to discover, so the empty merge is the shape that has to survive: it must
	// not emit `null` for the collections, nor "" for the fields the schema constrains.
	// The schema side of the same contract is pinned in openapi/openapi-case-tests.yaml.
	Describe("JSON shape written to values", func() {
		encode := func(d clouddatav1.YandexCloudDiscoveryData) map[string]any {
			raw, err := json.Marshal(d)
			Expect(err).NotTo(HaveOccurred())

			var decoded map[string]any
			Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
			return decoded
		}

		It("emits only the type markers and the region for a cluster with no discovery data", func() {
			decoded := encode(MergeDiscoveryData(clouddatav1.YandexCloudDiscoveryData{}, clouddatav1.YandexCloudDiscoveryData{}))

			Expect(decoded).To(Equal(map[string]any{
				"apiVersion": clouddatav1.APIVersion,
				"kind":       clouddatav1.YandexCloudDiscoveryDataKind,
				"region":     clouddatav1.YandexCloudDiscoveryDataDefaultRegion,
			}))
		})

		It("never emits a null for the optional fields", func() {
			decoded := encode(MergeDiscoveryData(clouddatav1.YandexCloudDiscoveryData{}, clouddatav1.YandexCloudDiscoveryData{}))

			for key, value := range decoded {
				Expect(value).NotTo(BeNil(), "%s must be omitted rather than written as null", key)
			}
		})

		It("keeps a discovered false rather than omitting it", func() {
			decoded := encode(MergeDiscoveryData(
				clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(false)},
				clouddatav1.YandexCloudDiscoveryData{},
			))

			Expect(decoded).To(HaveKeyWithValue("shouldAssignPublicIPAddress", false))
		})

		It("emits every field once discovery data is available", func() {
			decoded := encode(MergeDiscoveryData(fullDiscoveryData(), clouddatav1.YandexCloudDiscoveryData{}))

			Expect(decoded).To(HaveKeyWithValue("routeTableID", "rt-new"))
			Expect(decoded).To(HaveKeyWithValue("defaultLbTargetGroupNetworkId", "net-new"))
			Expect(decoded).To(HaveKeyWithValue("internalNetworkIDs", []any{"net-new"}))
			Expect(decoded).To(HaveKeyWithValue("zones", []any{"ru-central1-a"}))
			Expect(decoded).To(HaveKeyWithValue("zoneToSubnetIdMap", map[string]any{"ru-central1-a": "subnet-new"}))
			Expect(decoded).To(HaveKeyWithValue("shouldAssignPublicIPAddress", true))
		})
	})
})
