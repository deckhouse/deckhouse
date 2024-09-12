{{ define "getApiVersion" }}
  {{- if .Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy" }}
apiVersion: admissionregistration.k8s.io/v1
  {{- else if .Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1beta1/ValidatingAdmissionPolicy" }}
  apiVersion: admissionregistration.k8s.io/v1beta1
  {{- else }}
apiVersion: admissionregistration.k8s.io/v1alpha1
  {{- end }}
{{- end }}

