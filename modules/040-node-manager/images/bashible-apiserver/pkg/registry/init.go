package registry

import (
	"k8s.io/apiserver/pkg/registry/rest"

	"bashible-apiserver/pkg/registry/bashible/bashible"
	"bashible-apiserver/pkg/registry/bashible/kubernetesbundle"
	"bashible-apiserver/pkg/registry/bashible/nodegroupbundle"
	"bashible-apiserver/pkg/template"
)

func GetStorage(rootDir string, bashibleContext *template.Context, manager CachesManager) map[string]rest.Storage {

	v1alpha1storage := map[string]rest.Storage{}

	bashiblesStorage, err := bashible.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["bashibles"] = RESTInPeace(bashiblesStorage, err, manager.GetCache())

	k8sStorege, err := kubernetesbundle.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["kubernetesbundles"] = RESTInPeace(k8sStorege, err, manager.GetCache())

	ngStorage, err := nodegroupbundle.NewStorage(rootDir, bashibleContext)
	v1alpha1storage["nodegroupbundles"] = RESTInPeace(ngStorage, err, manager.GetCache())

	return v1alpha1storage
}
