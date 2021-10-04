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

package snapshot

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type PvcTermination struct {
	Name          string
	IsTerminating bool
}

// Index returns parsed smoke-mini index
func (pt PvcTermination) Index() Index {
	return IndexFromPVCName(pt.Name)
}

func NewPvcTermination(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ret := PvcTermination{
		Name:          obj.GetName(),
		IsTerminating: obj.GetDeletionTimestamp() != nil,
	}
	return ret, nil
}

type StorageClass struct {
	Name    string
	Default bool
}

func NewStorageClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		anno     = obj.GetAnnotations()
		inBeta   = anno["storageclass.beta.kubernetes.io/is-default-class"] == "true"
		inStable = anno["storageclass.kubernetes.io/is-default-class"] == "true"
	)
	sc := StorageClass{
		Name:    obj.GetName(),
		Default: inBeta || inStable,
	}
	return sc, nil
}
