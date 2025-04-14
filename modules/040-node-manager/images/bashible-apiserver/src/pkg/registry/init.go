/*
Copyright 2023 Flant JSC

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

package registry

import (
	"k8s.io/apiserver/pkg/registry/rest"

	"bashible-apiserver/pkg/registry/bashible/bashible"
	"bashible-apiserver/pkg/registry/bashible/bootstrap"
	"bashible-apiserver/pkg/registry/bashible/nodegroupbundle"
	"bashible-apiserver/pkg/template"
)

func GetStorage(rootDir string, bashibleContext *template.BashibleContext, stepsStorage *template.StepsStorage, manager CachesManager) map[string]rest.Storage {
	v1alpha1storage := map[string]rest.Storage{}

	bashiblesStorage, err := bashible.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["bashibles"] = RESTInPeace(bashiblesStorage, err, manager.GetCache())

	ngStorage, err := nodegroupbundle.NewStorage(rootDir, stepsStorage, bashibleContext)
	v1alpha1storage["nodegroupbundles"] = RESTInPeace(ngStorage, err, manager.GetCache())

	bootstrapStorage, err := bootstrap.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["bootstrap"] = RESTBootstrapInPeace(bootstrapStorage, err, manager.GetCache())

	return v1alpha1storage
}
