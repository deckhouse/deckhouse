package webhook

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func OldNewCSValidator(log logr.Logger) *CSValidator {
	return &CSValidator{
		log: log,
	}
}

type CSValidator struct {
	log logr.Logger
}

func (v *CSValidator) ValidateCreate(_ context.Context, object runtime.Object) (admission.Warnings, error) {
	//fmt.Println("CREATE LVM Change Request")
	//
	//fmt.Printf("%v", object)
	//fmt.Println(object)
	//f := object.GetObjectKind()
	//fmt.Println(f.GroupVersionKind().String())
	//
	//obj, ok := object.(*v1alpha1.LVMChangeRequest)
	//if ok {
	//	fmt.Println(obj)
	//}
	//
	//out := admission.Warnings{"AAAA", "BBBB", "CCCCC"}
	return nil, nil
}

func (v *CSValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	//newSC, ok := newObj.(*v1storage.StorageClass)
	//if !ok {
	//	return nil, fmt.Errorf("expected a new StorageClass but got a %T\", newObj")
	//}
	//oldSC, ok := oldObj.(*v1storage.StorageClass)
	//if !ok {
	//	return nil, fmt.Errorf("expected  old StorageClass but got a %T\", oldObj")
	//}
	//v.log.Info("Validation StorageClass", "old", oldSC.Size(), newSC.Size())

	//v.log.Info("Update StorageClass")
	//fmt.Println("Update StorageClass")
	//fmt.Println("UPDATE LVM Change Request")
	//fmt.Println(oldObj.GetObjectKind())
	//out := admission.Warnings{"AAAA", "BBBB", "CCCCC"}
	return nil, nil
}

func (v *CSValidator) ValidateDelete(_ context.Context, object runtime.Object) (admission.Warnings, error) {
	//fmt.Println("DELETE LVM Change Request")
	//fmt.Println(object.GetObjectKind())
	//out := admission.Warnings{"AAAA", "BBBB", "CCCCC"}
	return nil, nil
}

func (v *CSValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	fmt.Println("---------------------")
	fmt.Println(req.UserInfo)
	fmt.Println("---------------------")
	return admission.Response{}
}
