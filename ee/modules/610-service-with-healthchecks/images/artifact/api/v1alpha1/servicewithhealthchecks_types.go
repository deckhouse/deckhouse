/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServiceWithHealthchecksSpec defines the desired state of ServiceWithHealthchecks
type ServiceWithHealthchecksSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	corev1.ServiceSpec `json:",inline"`
	Healthcheck        Healthcheck `json:"healthcheck"`
}

// ServiceWithHealthchecksStatus defines the observed state of ServiceWithHealthchecks
type ServiceWithHealthchecksStatus struct {
	// LoadBalancer contains the current status of the load-balancer,
	// if one is present.
	// +optional
	LoadBalancer         corev1.LoadBalancerStatus `json:"loadBalancer,omitempty" protobuf:"bytes,1,opt,name=loadBalancer"`
	HealthcheckCondition HealthcheckCondition      `json:"healthcheckCondition,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=healthcheckCondition"`
	// Current service state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,3,rep,name=conditions"`
	// Current service state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=podName
	EndpointStatuses []EndpointStatus `json:"endpointStatuses,omitempty" patchStrategy:"merge" patchMergeKey:"podName" protobuf:"bytes,4,rep,name=endpointStatuses"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ServiceWithHealthchecks is the Schema for the servicewithhealthchecks API
type ServiceWithHealthchecks struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceWithHealthchecksSpec   `json:"spec,omitempty"`
	Status ServiceWithHealthchecksStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceWithHealthchecksList contains a list of ServiceWithHealthchecks
type ServiceWithHealthchecksList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceWithHealthchecks `json:"items"`
}

type Healthcheck struct {
	InitialDelaySeconds int32   `json:"initialDelaySeconds,omitempty" protobuf:"varint,1,opt,name=initialDelaySeconds"`
	PeriodSeconds       int32   `json:"periodSeconds,omitempty" protobuf:"varint,2,opt,name=periodSeconds"`
	Probes              []Probe `json:"probes,omitempty" protobuf:"bytes,3,rep,name=probes"`
}

type Probe struct {
	Mode             string        `json:"mode,omitempty" protobuf:"bytes,1,opt,name=mode"`
	TimeoutSeconds   int32         `json:"timeoutSeconds,omitempty" protobuf:"varint,2,opt,name=timeoutSeconds"`
	SuccessThreshold int32         `json:"successThreshold,omitempty" protobuf:"varint,3,opt,name=successThreshold"`
	FailureThreshold int32         `json:"failureThreshold,omitempty" protobuf:"varint,4,opt,name=failureThreshold"`
	HTTPHandler      *HTTPHandler  `json:"http,omitempty" protobuf:"bytes,5,opt,name=http"`
	TCPHandler       *TCPHandler   `json:"tcp,omitempty" protobuf:"bytes,6,opt,name=tcp"`
	PostgreSQL       *PGSQLHandler `json:"postgreSQL,omitempty" protobuf:"bytes,7,opt,name=postgreSQL"`
}

type PGSQLHandler struct {
	TargetPort intstr.IntOrString `json:"targetPort" protobuf:"bytes,1,opt,name=targetPort"`
	DBName     string             `json:"dbName,omitempty" protobuf:"bytes,2,opt,name=dbName"`
	// +default:value="select 1;"
	Query          string `json:"query,omitempty" protobuf:"bytes,3,opt,name=query"`
	AuthSecretName string `json:"authSecretName,omitempty" protobuf:"bytes,4,opt,name=authSecretName"`
}

type HTTPHandler struct {
	// Path to access on the HTTP server.
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,1,opt,name=path"`
	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	TargetPort intstr.IntOrString `json:"targetPort" protobuf:"bytes,2,opt,name=targetPort"`
	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	// +optional
	Host string `json:"host,omitempty" protobuf:"bytes,3,opt,name=host"`
	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	// +optional
	Scheme corev1.URIScheme `json:"scheme,omitempty" protobuf:"bytes,4,opt,name=scheme,casttype=URIScheme"`
	// Method to use for the request. Defaults to GET.
	// +optional
	Method string `json:"method,omitempty" protobuf:"bytes,5,opt,name=method"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	HTTPHeaders []corev1.HTTPHeader `json:"httpHeaders,omitempty" protobuf:"bytes,6,rep,name=httpHeaders"`
}

type TCPHandler struct {
	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	TargetPort intstr.IntOrString `json:"targetPort" protobuf:"bytes,1,opt,name=targetPort"`
}

type HealthcheckCondition struct {
	ObservedGeneration int64  `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`
	ActiveNodeName     string `json:"activeNodeName,omitempty" protobuf:"bytes,2,opt,name=activeNodeName"`
	Endpoints          int32  `json:"endpoints,omitempty" protobuf:"varint,3,opt,name=endpoints"`
	ReadyEndpoints     int32  `json:"readyEndpoints,omitempty" protobuf:"varint,4,opt,name=readyEndpoints"`
}

type EndpointStatus struct {
	// +kubebuilder:validation:Required
	PodName          string   `json:"podName" protobuf:"bytes,1,opt,name=podName"`
	NodeName         string   `json:"nodeName,omitempty" protobuf:"bytes,2,opt,name=nodeName"`
	Ready            bool     `json:"ready,omitempty" protobuf:"varint,3,opt,name=ready"`
	ProbesSuccessful bool     `json:"probesSuccessful,omitempty" protobuf:"varint,4,opt,name=probesSuccessful"`
	FailedProbes     []string `json:"failedProbes,omitempty" protobuf:"bytes,5,rep,name=failedProbes"`
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty" protobuf:"bytes,6,opt,name=lastProbeTime"`
}

func init() {
	SchemeBuilder.Register(&ServiceWithHealthchecks{}, &ServiceWithHealthchecksList{})
}
