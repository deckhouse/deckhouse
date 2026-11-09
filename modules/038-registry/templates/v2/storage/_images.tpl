{{- /*
  The images of the storage pod, by the name of the container that runs each.

  One list for two readers: the StatefulSet runs these images, and the image holder beside it
  keeps the same ones pulled on every master. The storage update controller replaces the only
  replica of an air-gapped cache only once the holder on its node runs every image of the pod
  template, compared reference for reference — so the two must never name an image differently.
  Written once here, they cannot; a container added to the pod without its image added here
  fails the template test that compares the two, rather than leaving the update waiting on a
  holder that will never hold it.

  Usage: {{- $images := include "registry_storage_images" $ | fromYaml }}
*/ -}}
{{- define "registry_storage_images" -}}
distribution: {{ include "helm_lib_module_image" (list . "dockerDistribution") | quote }}
auth: {{ include "helm_lib_module_image" (list . "dockerAuth") | quote }}
kube-rbac-proxy: {{ include "helm_lib_module_common_image" (list . "kubeRbacProxy") | quote }}
syncer: {{ include "helm_lib_module_image" (list . "registrySyncer") | quote }}
{{- end -}}
