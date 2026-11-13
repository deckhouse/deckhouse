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

package apiserver

import (
	"reflect"

	"k8s.io/apimachinery/pkg/version"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/util"
	"k8s.io/kube-openapi/pkg/validation/spec"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"
)

// openAPIDefinitions describes every type this server can serve. RegistryImage and its list are
// described field by field, so that `kubectl explain registryimages` says what each field means — the
// two sizes above all, which read alike and are not. The rest — the Status, the discovery types and
// /version, all of which the generic server carries and whose models the spec builder must find, or
// it stops the process — are free-form objects.
func openAPIDefinitions() openapicommon.GetOpenAPIDefinitions {
	freeForm := openapicommon.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"object"}},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{"x-kubernetes-preserve-unknown-fields": true},
			},
		},
	}

	return func(ref openapicommon.ReferenceCallback) map[string]openapicommon.OpenAPIDefinition {
		definitions := map[string]openapicommon.OpenAPIDefinition{}
		for _, known := range Scheme.AllKnownTypes() {
			definitions[util.GetCanonicalTypeName(reflect.New(known).Interface())] = freeForm
		}
		definitions[util.GetCanonicalTypeName(&version.Info{})] = freeForm

		image := util.GetCanonicalTypeName(&registryimagev1alpha1.RegistryImage{})
		definitions[image] = registryImageDefinition()
		definitions[util.GetCanonicalTypeName(&registryimagev1alpha1.RegistryImageList{})] = openapicommon.OpenAPIDefinition{
			Schema: spec.Schema{SchemaProps: spec.SchemaProps{
				Description: "A list of RegistryImage.",
				Type:        []string{"object"},
				Required:    []string{"items"},
				Properties: map[string]spec.Schema{
					"apiVersion": stringProperty("APIVersion of the list."),
					"kind":       stringProperty("Kind of the list."),
					"metadata":   objectProperty("Standard list metadata."),
					"items": {SchemaProps: spec.SchemaProps{
						Description: "The images.",
						Type:        []string{"array"},
						Items:       &spec.SchemaOrArray{Schema: &spec.Schema{SchemaProps: spec.SchemaProps{Ref: ref(image)}}},
					}},
				},
			}},
			Dependencies: []string{image},
		}
		return definitions
	}
}

func stringProperty(description string) spec.Schema {
	return spec.Schema{SchemaProps: spec.SchemaProps{Description: description, Type: []string{"string"}}}
}

func objectProperty(description string) spec.Schema {
	return spec.Schema{
		SchemaProps:      spec.SchemaProps{Description: description, Type: []string{"object"}},
		VendorExtensible: spec.VendorExtensible{Extensions: spec.Extensions{"x-kubernetes-preserve-unknown-fields": true}},
	}
}

func integerProperty(description string) spec.Schema {
	return spec.Schema{SchemaProps: spec.SchemaProps{Description: description, Type: []string{"integer"}, Format: "int64"}}
}

func enumProperty(description string, values ...string) spec.Schema {
	enum := make([]interface{}, 0, len(values))
	for _, value := range values {
		enum = append(enum, value)
	}
	return spec.Schema{SchemaProps: spec.SchemaProps{Description: description, Type: []string{"string"}, Enum: enum}}
}

func arrayOf(description string, item spec.Schema) spec.Schema {
	return spec.Schema{SchemaProps: spec.SchemaProps{
		Description: description,
		Type:        []string{"array"},
		Items:       &spec.SchemaOrArray{Schema: &item},
	}}
}

func states(description string) spec.Schema {
	return enumProperty(description, "Present", "Missing", "Incomplete", "Corrupted", "Unknown")
}

// stateMeaning says what each state means, for every field that carries one.
const stateMeaning = `

- Present: the replica holds the manifest and every blob the image consists of.
- Missing: an image of the set the replica does not hold at all.
- Incomplete: the replica holds the manifest without some of the blobs it names; a pull-through cache leaves this after it served the manifest alone.
- Corrupted: a blob of the image does not match its digest. The module repairs it on its own with a RegistryOperation of type Repair.
- Unknown: the replica did not answer, or has not read its store since it started.`

func registryImageDefinition() openapicommon.OpenAPIDefinition {
	owner := spec.Schema{SchemaProps: spec.SchemaProps{
		Type:     []string{"object"},
		Required: []string{"kind"},
		Properties: map[string]spec.Schema{
			"kind": enumProperty(`What declares the image.

- Platform: an image of the platform, declared by a Deckhouse release.
- Release: the image of a Deckhouse release itself, by its version tag.
- Module: an image of a module, or the module's package.
- Catalogue: an entry of a catalogue of modules: a release channel of a module the platform reads to know what it can install.`,
				"Platform", "Release", "Module", "Catalogue"),
			"module":   stringProperty("The module the image belongs to."),
			"image":    stringProperty("The name of the image within the module."),
			"versions": arrayOf("The Deckhouse releases that declare the image, or the versions of the module.", stringProperty("")),
			"source":   stringProperty("The ModuleSource the module comes from. Empty for the platform's own modules."),
		},
	}}
	replica := spec.Schema{SchemaProps: spec.SchemaProps{
		Type:     []string{"object"},
		Required: []string{"node", "state"},
		Properties: map[string]spec.Schema{
			"node":      stringProperty("The node the replica runs on."),
			"state":     states("The state of the image on this replica." + stateMeaning),
			"message":   stringProperty("What is wrong when the state is not Present: the blobs that are missing or broken, or why the replica did not answer."),
			"indexedAt": {SchemaProps: spec.SchemaProps{Description: "When the replica last read its store. The state is as of then; a replica reads it every two minutes, and at once after an operation.", Type: []string{"string"}, Format: "date-time"}},
		},
	}}

	return openapicommon.OpenAPIDefinition{Schema: spec.Schema{SchemaProps: spec.SchemaProps{
		Description: `One image of the in-cluster registry cache, by the digest of its manifest, across the replicas of the cache.

Not stored: every request asks each replica what its store holds. Read-only. To check images against their digests, or to replace broken blobs with whole copies, create a RegistryOperation of type Verify or Repair.`,
		Type:     []string{"object"},
		Required: []string{"digest", "inSet", "state"},
		Properties: map[string]spec.Schema{
			"apiVersion": stringProperty("registry.deckhouse.io/v1alpha1."),
			"kind":       stringProperty("RegistryImage."),
			"metadata": objectProperty(`The name is the digest with a dash for the colon, for example sha256-0123….

Labels to select by:
- registry.deckhouse.io/in-set: "true" for an image the cluster declares, "false" for one it does not.
- registry.deckhouse.io/state: the state of the image, as in state.
- registry.deckhouse.io/module: the module the image belongs to; the first by name when it belongs to several. A selector by it matches any of them, and one that rules a module out (!=, notin) skips every image that module shares.
- registry.deckhouse.io/owner-kind: the kind of the first owner.`),
			"digest": stringProperty("The digest of the image's manifest."),
			"inSet": {SchemaProps: spec.SchemaProps{
				Description: "Whether a release or a module of the cluster declares the image. An image outside the set was pulled through the cache on request, or was left by an older release; the garbage collection removes it.",
				Type:        []string{"boolean"},
			}},
			"state":        states("The worst state of the image among the replicas." + stateMeaning),
			"owners":       arrayOf("Every place the image is used: releases, modules and images. Empty outside the set.", owner),
			"repositories": arrayOf("Where in the cache the image is linked from.", stringProperty("")),
			"mediaType":    stringProperty("The media type of the manifest: an image, or an index of images for several platforms."),
			"size": integerProperty(`The full size of the image, in bytes: its manifest and every layer it consists of, each counted once.

An image is made of layers, and a layer several images share is stored once. So the sizes of all images add up to more than the cache takes on disk.`),
			"reclaimableSize": integerProperty(`How many bytes the cache would free on a replica if this image alone were removed: its layers that no other image references.

Usually close to size: the images of the platform share few layers.`),
			"replicas": arrayOf("The image on each replica of the cache.", replica),
		},
	}}}
}
