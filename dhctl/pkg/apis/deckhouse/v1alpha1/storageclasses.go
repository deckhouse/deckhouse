/*
Copyright 2025 Flant JSC

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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis"
)

var (
	LocalStorageClassGVR              = storageClassGVR("localstorageclasses")
	ReplicatedStorageClassGVR         = storageClassGVR("replicatedstorageclasses")
	NFSStorageClassGVR                = storageClassGVR("nfsstorageclasses")
	CephStorageClassGVR               = storageClassGVR("cephstorageclasses")
	SCSIStorageClassGVR               = storageClassGVR("scsistorageclasses")
	S3StorageClassGVR                 = storageClassGVR("s3storageclasses")
	YadroTatlinUnifiedStorageClassGVR = storageClassGVR("yadrotatlinunifiedstorageclasses")
	NetappStorageClassGVR             = storageClassGVR("netappstorageclasses")
	HuaweiStorageClassGVR             = storageClassGVR("huaweistorageclasses")
	HPEStorageClassGVR                = storageClassGVR("hpestorageclasses")

	d8StoragesListKindToGVR = apis.ListKindToGVR{
		"LocalStorageClassList":              LocalStorageClassGVR,
		"ReplicatedStorageClassList":         ReplicatedStorageClassGVR,
		"NfsStorageClassList":                NFSStorageClassGVR,
		"CephStorageClassList":               CephStorageClassGVR,
		"SCSIStorageClassList":               SCSIStorageClassGVR,
		"S3StorageClassList":                 S3StorageClassGVR,
		"YadroTatlinUnifiedStorageClassList": YadroTatlinUnifiedStorageClassGVR,
		"NetappStorageClassList":             NetappStorageClassGVR,
		"HuaweiStorageClassList":             HuaweiStorageClassGVR,
		"HPEStorageClassList":                HPEStorageClassGVR,
	}
)

func storageClassGVR(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "storage.deckhouse.io",
		Version:  "v1alpha1",
		Resource: resource,
	}
}

func D8StoragesGVRs() []schema.GroupVersionResource {
	return apis.GVRList(d8StoragesListKindToGVR)
}

func D8StoragesListsGVRs() apis.ListKindToGVR {
	return apis.CopyListKindToGVR(d8StoragesListKindToGVR)
}
