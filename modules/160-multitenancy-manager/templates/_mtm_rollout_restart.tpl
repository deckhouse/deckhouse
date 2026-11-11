{{/*
A rollout restart is the one manual change the heritage policy and the manifests policy allow.
It is not a match condition: a match condition that is false skips the policy, so any other change
sent in the same request would be skipped with the annotation. The validation allows the request
only when the new object equals the old one apart from kubectl.kubernetes.io/restartedAt on the
pod template. resourceVersion, generation and managedFields are the fields the API server writes
on the update itself, including the generation bump of a pod-template change, so they are not part
of the comparison. Every other field is: spec apart from the template, the template apart from its
metadata, that metadata apart from its annotations, the annotations apart from the one key, the
object's metadata apart from the three API-server fields, and every remaining field, status
included. Subresources are not part of this object.
*/}}
{{- define "mtm.rollout_restart_variables" }}
- name: hasPodTemplate
  expression: 'object != null && oldObject != null && has(object.spec) && has(object.spec.template) && has(object.spec.template.metadata) && has(oldObject.spec) && has(oldObject.spec.template) && has(oldObject.spec.template.metadata)'
- name: oldPodAnnotations
  expression: 'variables.hasPodTemplate && has(oldObject.spec.template.metadata.annotations) ? oldObject.spec.template.metadata.annotations : {}'
- name: newPodAnnotations
  expression: 'variables.hasPodTemplate && has(object.spec.template.metadata.annotations) ? object.spec.template.metadata.annotations : {}'
- name: restartedAtChanged
  expression: 'variables.hasPodTemplate && ("kubectl.kubernetes.io/restartedAt" in variables.newPodAnnotations) && (!("kubectl.kubernetes.io/restartedAt" in variables.oldPodAnnotations) || variables.oldPodAnnotations["kubectl.kubernetes.io/restartedAt"] != variables.newPodAnnotations["kubectl.kubernetes.io/restartedAt"])'
- name: podAnnotationsOtherwiseEqual
  expression: 'variables.oldPodAnnotations.all(k, k == "kubectl.kubernetes.io/restartedAt" || (k in variables.newPodAnnotations && variables.oldPodAnnotations[k] == variables.newPodAnnotations[k])) && variables.newPodAnnotations.all(k, k == "kubectl.kubernetes.io/restartedAt" || k in variables.oldPodAnnotations)'
- name: templateMetadataOtherwiseEqual
  expression: '!variables.hasPodTemplate || (oldObject.spec.template.metadata.all(k, k == "annotations" || (k in object.spec.template.metadata && oldObject.spec.template.metadata[k] == object.spec.template.metadata[k])) && object.spec.template.metadata.all(k, k == "annotations" || k in oldObject.spec.template.metadata))'
- name: templateOtherwiseEqual
  expression: '!variables.hasPodTemplate || (oldObject.spec.template.all(k, k == "metadata" || (k in object.spec.template && oldObject.spec.template[k] == object.spec.template[k])) && object.spec.template.all(k, k == "metadata" || k in oldObject.spec.template))'
- name: specOtherwiseEqual
  expression: '!variables.hasPodTemplate || (oldObject.spec.all(k, k == "template" || (k in object.spec && oldObject.spec[k] == object.spec[k])) && object.spec.all(k, k == "template" || k in oldObject.spec))'
- name: metadataOtherwiseEqual
  expression: 'object == null || oldObject == null || ((has(oldObject.metadata) ? oldObject.metadata : {}).all(k, k in ["resourceVersion", "generation", "managedFields"] || (has(object.metadata) && k in object.metadata && oldObject.metadata[k] == object.metadata[k])) && (has(object.metadata) ? object.metadata : {}).all(k, k in ["resourceVersion", "generation", "managedFields"] || (has(oldObject.metadata) && k in oldObject.metadata)))'
- name: objectOtherwiseEqual
  expression: 'object == null || oldObject == null || (oldObject.all(k, k == "metadata" || k == "spec" || (k in object && oldObject[k] == object[k])) && object.all(k, k == "metadata" || k == "spec" || k in oldObject))'
- name: rolloutRestart
  expression: 'variables.restartedAtChanged && variables.podAnnotationsOtherwiseEqual && variables.templateMetadataOtherwiseEqual && variables.templateOtherwiseEqual && variables.specOtherwiseEqual && variables.metadataOtherwiseEqual && variables.objectOtherwiseEqual'
{{- end }}
