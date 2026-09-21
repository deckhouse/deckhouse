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

// ResolveDiscoveryData picks one payload and hands it to SetDefaults, which is the only
// transformation applied on the way to cloudProviderYandex.internal.providerDiscoveryData.
var _ = Describe("discovery data defaulting", func() {
	withDefaults := func(d clouddatav1.YandexCloudDiscoveryData) clouddatav1.YandexCloudDiscoveryData {
		d.SetDefaults()
		return d
	}

	fullDiscoveryData := func() clouddatav1.YandexCloudDiscoveryData {
		return clouddatav1.YandexCloudDiscoveryData{
			RouteTableID:                  "rt-discovered",
			DefaultLbTargetGroupNetworkID: "net-discovered",
			InternalNetworkIDs:            []string{"net-discovered"},
			Zones:                         []string{"ru-central1-a"},
			ZoneToSubnetIDMap:             map[string]string{"ru-central1-a": "subnet-discovered"},
			ShouldAssignPublicIPAddress:   ptr.To(true),
			NATInstanceName:               "nat-discovered",
			NATInstanceZone:               "ru-central1-a",
			MonitoringAPIKey:              "key-discovered",
		}
	}

	It("fills the type markers and the region on an empty payload", func() {
		result := withDefaults(clouddatav1.YandexCloudDiscoveryData{})

		Expect(result.APIVersion).To(Equal(clouddatav1.APIVersion))
		Expect(result.Kind).To(Equal(clouddatav1.YandexCloudDiscoveryDataKind))
		Expect(result.Region).To(Equal(clouddatav1.YandexCloudDiscoveryDataDefaultRegion))
	})

	// Every field is defaulted rather than overwritten: a payload that carries a value states a
	// fact about the infrastructure that was actually created, and rewriting it would hide a real
	// mismatch instead of surfacing it.
	It("keeps the values a payload already carries", func() {
		result := withDefaults(clouddatav1.YandexCloudDiscoveryData{
			APIVersion: "deckhouse.io/v2",
			Kind:       "SomeOtherKind",
			Region:     "ru-central2",
		})

		Expect(result.APIVersion).To(Equal("deckhouse.io/v2"))
		Expect(result.Kind).To(Equal("SomeOtherKind"))
		Expect(result.Region).To(Equal("ru-central2"))
	})

	It("leaves every discovered field untouched", func() {
		result := withDefaults(fullDiscoveryData())

		Expect(result.RouteTableID).To(Equal("rt-discovered"))
		Expect(result.DefaultLbTargetGroupNetworkID).To(Equal("net-discovered"))
		Expect(result.InternalNetworkIDs).To(Equal([]string{"net-discovered"}))
		Expect(result.Zones).To(Equal([]string{"ru-central1-a"}))
		Expect(result.ZoneToSubnetIDMap).To(Equal(map[string]string{"ru-central1-a": "subnet-discovered"}))
		Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeTrue()))
		Expect(result.NATInstanceName).To(Equal("nat-discovered"))
		Expect(result.NATInstanceZone).To(Equal("ru-central1-a"))
		Expect(result.MonitoringAPIKey).To(Equal("key-discovered"))
	})

	// shouldAssignPublicIPAddress is a pointer precisely so that "not discovered yet" and
	// "discovered as false" stay distinguishable. With a plain bool the WithoutNAT-only `true`
	// could never be turned back off, and a discovered `false` was indistinguishable from an
	// absent value — which is what put a bare `false` into values for a cluster that had no
	// discovery data at all.
	Describe("shouldAssignPublicIPAddress", func() {
		It("keeps a discovered false", func() {
			result := withDefaults(clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(false)})

			Expect(result.ShouldAssignPublicIPAddress).To(HaveValue(BeFalse()))
		})

		It("stays unset when the payload does not carry it", func() {
			result := withDefaults(clouddatav1.YandexCloudDiscoveryData{})

			Expect(result.ShouldAssignPublicIPAddress).To(BeNil())
		})
	})

	// The hook writes the resolved struct straight into
	// cloudProviderYandex.internal.providerDiscoveryData, and that path is validated against
	// openapi/values.yaml on every write. A cluster whose infrastructure DKP does not create has
	// nothing to discover, so the empty payload is the shape that has to survive: it must not emit
	// `null` for the collections, nor "" for the fields the schema constrains. The schema side of
	// the same contract is pinned in openapi/openapi-case-tests.yaml.
	Describe("JSON shape written to values", func() {
		encode := func(d clouddatav1.YandexCloudDiscoveryData) map[string]any {
			raw, err := json.Marshal(d)
			Expect(err).NotTo(HaveOccurred())

			var decoded map[string]any
			Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
			return decoded
		}

		It("emits only the type markers and the region for a cluster with no discovery data", func() {
			decoded := encode(withDefaults(clouddatav1.YandexCloudDiscoveryData{}))

			Expect(decoded).To(Equal(map[string]any{
				"apiVersion": clouddatav1.APIVersion,
				"kind":       clouddatav1.YandexCloudDiscoveryDataKind,
				"region":     clouddatav1.YandexCloudDiscoveryDataDefaultRegion,
			}))
		})

		It("never emits a null for the optional fields", func() {
			decoded := encode(withDefaults(clouddatav1.YandexCloudDiscoveryData{}))

			for key, value := range decoded {
				Expect(value).NotTo(BeNil(), "%s must be omitted rather than written as null", key)
			}
		})

		It("keeps a discovered false rather than omitting it", func() {
			decoded := encode(withDefaults(clouddatav1.YandexCloudDiscoveryData{ShouldAssignPublicIPAddress: ptr.To(false)}))

			Expect(decoded).To(HaveKeyWithValue("shouldAssignPublicIPAddress", false))
		})

		It("emits every field once discovery data is available", func() {
			decoded := encode(withDefaults(fullDiscoveryData()))

			Expect(decoded).To(HaveKeyWithValue("routeTableID", "rt-discovered"))
			Expect(decoded).To(HaveKeyWithValue("defaultLbTargetGroupNetworkId", "net-discovered"))
			Expect(decoded).To(HaveKeyWithValue("internalNetworkIDs", []any{"net-discovered"}))
			Expect(decoded).To(HaveKeyWithValue("zones", []any{"ru-central1-a"}))
			Expect(decoded).To(HaveKeyWithValue("zoneToSubnetIdMap", map[string]any{"ru-central1-a": "subnet-discovered"}))
			Expect(decoded).To(HaveKeyWithValue("shouldAssignPublicIPAddress", true))
		})
	})
})
