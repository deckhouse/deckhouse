package registry

import (
	"k8s.io/apiserver/pkg/registry/rest"

	"d8.io/bashible/pkg/registry/bashible/bashible"
	"d8.io/bashible/pkg/registry/bashible/nodegroupbundle"
	"d8.io/bashible/pkg/template"
)

func GetStorage(rootDir string, bashibleContext *template.BashibleContext, manager CachesManager) map[string]rest.Storage {
	v1alpha1storage := map[string]rest.Storage{}

	bashiblesStorage, err := bashible.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["bashibles"] = RESTInPeace(bashiblesStorage, err, manager.GetCache())

	ngStorage, err := nodegroupbundle.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["nodegroupbundles"] = RESTInPeace(ngStorage, err, manager.GetCache())

	return v1alpha1storage
}
