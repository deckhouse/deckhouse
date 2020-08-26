package crd

import (
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "upmeter/pkg/crd/v1"
	"upmeter/pkg/probe/types"
)

func ConvertToDowntimeIncidents(obj *unstructured.Unstructured) []types.DowntimeIncident {
	var incidentObj v1.Downtime
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(obj.UnstructuredContent(), &incidentObj)
	if err != nil {
		log.Errorf("convert Unstructured to Downtime: %v", err)
	}

	return incidentObj.GetDowntimeIncidents()
}
