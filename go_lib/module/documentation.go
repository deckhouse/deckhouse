package module

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// UpdateDocumentation creates or updates ModuleDocumentation custom resource
func UpdateDocumentation(
	ctx context.Context,
	client client.Client,
	moduleName, moduleVersion, moduleChecksum, modulePath, moduleSource string,
	ownerRef metav1.OwnerReference,
) error {
	var md v1alpha1.ModuleDocumentation
	err := client.Get(ctx, types.NamespacedName{Name: moduleName}, &md)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// just create
			md = v1alpha1.ModuleDocumentation{
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ModuleDocumentationGVK.Kind,
					APIVersion: v1alpha1.ModuleDocumentationGVK.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
					Labels: map[string]string{
						"module": moduleName,
						"source": moduleSource,
					},
					OwnerReferences: []metav1.OwnerReference{ownerRef},
				},
				Spec: v1alpha1.ModuleDocumentationSpec{
					Version:  moduleVersion,
					Path:     modulePath,
					Checksum: moduleChecksum,
				},
			}
			err = client.Create(ctx, &md)
			if err != nil {
				return err
			}
		}
		return err
	}

	if md.Spec.Version != moduleVersion || md.Spec.Checksum != moduleChecksum {
		// update CR
		md.Spec.Path = modulePath
		md.Spec.Version = moduleVersion
		md.Spec.Checksum = moduleChecksum
		md.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

		err = client.Update(ctx, &md)
		if err != nil {
			return err
		}
	}

	return nil
}
