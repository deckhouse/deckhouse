package webhook

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	v1 "k8s.io/api/admission/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	AdmissionKind       = "AdmissionReview"
	AdmissionAPIVersion = "admission.k8s.io/v1"
	AllowedStorageClass = "linstor.csi.linbit.com"
	AllowedUser         = "system:serviceaccount:d8-storage-configurator:storage-configurator"
	DeleteOperation     = "DELETE"
)

type AdmissionRequest struct {
	Kind       string            `json:"kind"`
	ApiVersion string            `json:"apiVersion"`
	Request    admission.Request `json:"request"`
}

type StorageClassHandler struct {
	Log logr.Logger
}

func NewAdmissionStorageClass(
	mgr manager.Manager,
) error {

	log := mgr.GetLogger()
	server := mgr.GetWebhookServer()

	server.Register("/validate-storage-class", StorageClassHandler{
		Log: log,
	})

	return nil
}

func (h StorageClassHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	h.Log.Info("run webhook StorageClass...")

	var ar AdmissionRequest
	var scObject storagev1.StorageClass
	var tmpRawRequest []byte

	buf, err := io.ReadAll(r.Body)
	if err != nil {
		h.Log.Error(err, "error read body admission request")
		return
	}

	err = json.Unmarshal(buf, &ar)
	if err != nil {
		h.Log.Error(err, "error decode json body admission request")
		return
	}

	if ar.Request.Operation == DeleteOperation {
		tmpRawRequest = ar.Request.OldObject.Raw
	} else {
		tmpRawRequest = ar.Request.Object.Raw
	}

	err = json.Unmarshal(tmpRawRequest, &scObject)
	if err != nil {
		h.Log.Error(err, "error unmarshal Object storage class")
		return
	}

	//h.Log.Info(string(ar.Request.Operation) + "  " + scObject.Name)

	admissionResponse := &v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       AdmissionKind,
			APIVersion: AdmissionAPIVersion,
		},
		Response: &v1.AdmissionResponse{
			UID:     ar.Request.UID,
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("Manual %s is prohibited. Please %s LinstorStorageClass %s", string(ar.Request.Operation), string(ar.Request.Operation), ar.Request.Name),
				Code:    http.StatusForbidden,
			},
			//Warnings: []string{fmt.Sprintf("Please %s LinstorStorageClass %s", string(ar.Request.Operation), ar.Request.Name)},
		},
	}

	if scObject.Provisioner != AllowedStorageClass {
		admissionResponse.Response.Allowed = true
		admissionResponse.Response.Result.Message = "Allowed"
		admissionResponse.Response.Result.Code = http.StatusOK
		admissionResponse.Response.Warnings = nil
	} else {
		if ar.Request.UserInfo.Username == AllowedUser {
			admissionResponse.Response.Allowed = true
			admissionResponse.Response.Result.Message = "Allowed"
			admissionResponse.Response.Result.Code = http.StatusOK
			admissionResponse.Response.Warnings = nil
		}
	}

	w.Header().Set("Content-Type", "application/json")

	wr, err := json.Marshal(admissionResponse)
	if err != nil {
		h.Log.Error(err, "error marshal response json")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(wr)
	if err != nil {
		return
	}
	h.Log.Info("web hook completed")
}
