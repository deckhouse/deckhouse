package v1alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func aggregateObjErrors(qualifiedKind schema.GroupKind, name string, errs field.ErrorList) (admission.Warnings, error) {
	if len(errs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		qualifiedKind,
		name,
		errs,
	)
}
