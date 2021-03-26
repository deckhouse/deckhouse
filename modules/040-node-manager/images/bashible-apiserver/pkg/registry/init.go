package registry

import (
	"bashible-apiserver/pkg/registry/bashible/bashible"
	"bashible-apiserver/pkg/registry/bashible/kubernetesbundle"
	"bashible-apiserver/pkg/registry/bashible/nodegroupbundle"
	"bashible-apiserver/pkg/template"

	"k8s.io/apiserver/pkg/registry/rest"
)

func GetStorage(rootDir string, bashibleContext *template.Context) map[string]rest.Storage {

	v1alpha1storage := map[string]rest.Storage{}

	v1alpha1storage["bashibles"] = RESTInPeace(bashible.NewStorage(rootDir, bashibleContext))
	v1alpha1storage["kubernetesbundles"] = RESTInPeace(kubernetesbundle.NewStorage(rootDir, bashibleContext))
	v1alpha1storage["nodegroupbundles"] = RESTInPeace(nodegroupbundle.NewStorage(rootDir, bashibleContext))

	return v1alpha1storage
}
