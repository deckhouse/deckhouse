/*
Copyright 2022 Flant JSC

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
	"reflect"
	"testing"

	lclient "github.com/LINBIT/golinstor/client"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseLVMThinPoolsEmpty(t *testing.T) {
	got, err := parseLVMThinPools("node1", ``)
	if err != nil {
		t.Errorf("\nexpected no error\ngot: %s", err.Error())
	}
	if got != nil {
		t.Errorf("\nexpected nil\ngot: %+v", got)
	}
}

func TestParseThinPoolsWrong(t *testing.T) {
	_, err := parseLVMThinPools("node1", `a;b`)
	if err == nil {
		t.Errorf("\nexpected error\ngot: nil")
	}
}

func TestParseThinPoolsNoTags(t *testing.T) {
	got, err := parseLVMThinPools("node1", `  data;linstor_data;twi---tz--;aDJhKS-fdhT-94VT-MxG8-8WMY-3SwO-2An0gR;
  pvc-ecc0e656-78ca-497f-8f7a-f9fe3b384748_00000;linstor_data;Vwi-aotz--;7hVfzc-HBLf-R2PB-Yo5L-BXvQ-30aa-GC5Ced;
  root;vg0;-wi-ao----;PGmBTo-G5Gp-kjKk-mIMv-hprr-BdPG-DPCJHP;`)
	if err != nil {
		t.Errorf("\nexpected no error\ngot: %s", err.Error())
	}
	expected := []Candidate{
		Candidate{
			Name:       "LVM Logical Volume linstor_data/data",
			UUID:       "aDJhKS-fdhT-94VT-MxG8-8WMY-3SwO-2An0gR",
			SkipReason: "has no propper tag set: can't find tag with prefix linstor",
		},
		Candidate{
			Name:       "LVM Logical Volume linstor_data/pvc-ecc0e656-78ca-497f-8f7a-f9fe3b384748_00000",
			UUID:       "7hVfzc-HBLf-R2PB-Yo5L-BXvQ-30aa-GC5Ced",
			SkipReason: "is not a thin pool",
		},
		Candidate{
			Name:       "LVM Logical Volume vg0/root",
			UUID:       "PGmBTo-G5Gp-kjKk-mIMv-hprr-BdPG-DPCJHP",
			SkipReason: "is not a thin pool",
		},
	}
	diffCandidates(t, &expected, &got)

}

func TestParseThinPoolsWithTags(t *testing.T) {
	got, _ := parseLVMThinPools("node1", `  data;linstor_data;twi---tz--;aDJhKS-fdhT-94VT-MxG8-8WMY-3SwO-2An0gR;linstor-ssd
  pvc-ecc0e656-78ca-497f-8f7a-f9fe3b384748_00000;linstor_data;Vwi-aotz--;7hVfzc-HBLf-R2PB-Yo5L-BXvQ-30aa-GC5Ced;linstor-ssd
  root;vg0;-wi-ao----;PGmBTo-G5Gp-kjKk-mIMv-hprr-BdPG-DPCJHP;`)
	expected := []Candidate{
		Candidate{
			Name: "LVM Logical Volume linstor_data/data",
			UUID: "aDJhKS-fdhT-94VT-MxG8-8WMY-3SwO-2An0gR",
			StoragePool: lclient.StoragePool{
				StoragePoolName: "ssd",
				ProviderKind:    lclient.LVM_THIN,
				NodeName:        "node1",
				Props: map[string]string{
					"StorDriver/LvmVg":    "linstor_data",
					"StorDriver/ThinPool": "data",
				},
			},
		},
		Candidate{
			Name:       "LVM Logical Volume linstor_data/pvc-ecc0e656-78ca-497f-8f7a-f9fe3b384748_00000",
			UUID:       "7hVfzc-HBLf-R2PB-Yo5L-BXvQ-30aa-GC5Ced",
			SkipReason: "is not a thin pool",
		},
		Candidate{
			Name:       "LVM Logical Volume vg0/root",
			UUID:       "PGmBTo-G5Gp-kjKk-mIMv-hprr-BdPG-DPCJHP",
			SkipReason: "is not a thin pool",
		},
	}
	diffCandidates(t, &expected, &got)
}

func TestParseVolumeGroupsEmpty(t *testing.T) {
	got, err := parseLVMVolumeGroups("node1", ``)
	if err != nil {
		t.Errorf("\nexpected no error\ngot: %s", err.Error())
	}
	if got != nil {
		t.Errorf("\nexpected nil\ngot: %+v", got)
	}
}

func TestParseVolumeGroupsWrong(t *testing.T) {
	_, err := parseLVMVolumeGroups("node1", `avasd`)
	if err == nil {
		t.Errorf("\nexpected error\ngot: nil")
	}
}

func TestParseVolumeGroupsNoTags(t *testing.T) {
	got, err := parseLVMVolumeGroups("node1", `  linstor_data;BQ5CtV-2arB-FUA8-oynj-XWk2-1pFa-urUSxO;
  vg0;hCbPFt-asAS-7DVb-OLtl-Ame3-XSmB-sxyXsO;`)
	if err != nil {
		t.Errorf("\nexpected no error\ngot: %s", err.Error())
	}
	expected := []Candidate{
		Candidate{
			Name:       "LVM Volume Group linstor_data",
			UUID:       "BQ5CtV-2arB-FUA8-oynj-XWk2-1pFa-urUSxO",
			SkipReason: "has no propper tag set: can't find tag with prefix linstor",
		},
		Candidate{
			Name:       "LVM Volume Group vg0",
			UUID:       "hCbPFt-asAS-7DVb-OLtl-Ame3-XSmB-sxyXsO",
			SkipReason: "has no propper tag set: can't find tag with prefix linstor",
		},
	}
	diffCandidates(t, &expected, &got)

}

func diffCandidates(t *testing.T, expected *[]Candidate, got *[]Candidate) {
	e := *expected
	g := *got
	for i := 0; i < len(e); i++ {
		if e[i].SkipReason != "" {
			g[i].StoragePool = lclient.StoragePool{}
		}
		if !reflect.DeepEqual(g[i], e[i]) {
			t.Errorf("\ncount:\t\t%d\nexpected:\t%+v\ngot:\t\t%+v", i+1, e[i], g[i])
		}
	}
}

func TestParseVolumeGroupsWithTags(t *testing.T) {
	got, err := parseLVMVolumeGroups("node1", `  linstor_data;BQ5CtV-2arB-FUA8-oynj-XWk2-1pFa-urUSxO;linstor-some-data
  vg0;hCbPFt-asAS-7DVb-OLtl-Ame3-XSmB-sxyXsO;`)
	if err != nil {
		t.Errorf("\nexpected no error\ngot: %s", err.Error())
	}
	expected := []Candidate{
		Candidate{
			Name: "LVM Volume Group linstor_data",
			UUID: "BQ5CtV-2arB-FUA8-oynj-XWk2-1pFa-urUSxO",
			StoragePool: lclient.StoragePool{
				StoragePoolName: "some-data",
				ProviderKind:    lclient.LVM,
				NodeName:        "node1",
				Props: map[string]string{
					"StorDriver/LvmVg": "linstor_data",
				},
			},
		},
		Candidate{
			Name:       "LVM Volume Group vg0",
			UUID:       "hCbPFt-asAS-7DVb-OLtl-Ame3-XSmB-sxyXsO",
			SkipReason: "has no propper tag set: can't find tag with prefix linstor",
		},
	}
	diffCandidates(t, &expected, &got)
}

func TestNewKubernetesStorageClasses(t *testing.T) {
	tp := lclient.StoragePool{
		StoragePoolName: "ssd",
		ProviderKind:    lclient.LVM_THIN,
		NodeName:        "node1",
		Props: map[string]string{
			"StorDriver/LvmVg":    "linstor_data",
			"StorDriver/ThinPool": "data",
		},
	}
	got := newKubernetesStorageClass(&tp, 2)

	volBindMode := storagev1.VolumeBindingImmediate
	allowVolumeExpansion := true
	reclaimPolicy := v1.PersistentVolumeReclaimDelete

	expected := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "linstor-ssd-r2",
		},
		Provisioner:          "linstor.csi.linbit.com",
		VolumeBindingMode:    &volBindMode,
		AllowVolumeExpansion: &allowVolumeExpansion,
		ReclaimPolicy:        &reclaimPolicy,
		Parameters: map[string]string{
			"linstor.csi.linbit.com/storagePool":    "ssd",
			"linstor.csi.linbit.com/placementCount": "2",
		},
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("\nexpected: %+v\ngot: %+v", expected, got)
	}
}
