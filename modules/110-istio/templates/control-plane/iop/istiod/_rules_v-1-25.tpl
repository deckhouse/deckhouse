{{- define "istiod_rules_v-1-25" -}}
- apiGroups:
  - apps
  resources:
  - deployments
  verbs:
  - get
  - list
  - watch
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - services
  - serviceaccounts
  verbs:
  - get
  - watch
  - list
  - update
  - patch
  - create
  - delete
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - mutatingwebhookconfigurations
  verbs:
  - get
  - list
  - watch
  - update
  - patch
- apiGroups:
  - admissionregistration.k8s.io
  resources:
  - validatingwebhookconfigurations
  verbs:
  - get
  - list
  - watch
  - update
- apiGroups:
  - security.istio.io
  resources:
  - authorizationpolicies
  - peerauthentications
  - requestauthentications
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - networking.istio.io
  resources:
  - destinationrules
  - envoyfilters
  - proxyconfigs
  - serviceentries
  - sidecars
  - virtualservices
  - workloadgroups
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - networking.istio.io
  resources:
  - serviceentries/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - telemetry.istio.io
  resources:
  - telemetries
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - extensions.istio.io
  resources:
  - wasmplugins
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - networking.istio.io
  resources:
  - workloadentries
  - gateways
  verbs:
  - get
  - watch
  - list
  - update
  - patch
  - create
  - delete
- apiGroups:
  - networking.istio.io
  resources:
  - workloadentries/status
  verbs:
  - get
  - watch
  - list
  - update
  - patch
  - create
  - delete
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  - namespaces
  - endpoints
  - replicationcontrollers
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  - ingressclasses
  - ingresses/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
  - deletecollection
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - delete
  - create
  - get
  - list
  - watch
  - update
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups:
  - authorization.k8s.io
  resources:
  - subjectaccessreviews
  verbs:
  - create
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gatewayclasses
  verbs:
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - create
  - update
  - delete
  - get
  - watch
  - list
- apiGroups:
  - multicluster.x-k8s.io
  resources:
  - serviceexports
  - serviceimports
  verbs:
  - get
  - watch
  - list
  - create
  - delete
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
  - deletecollection
- apiGroups:
  - networking.x-k8s.io
  resources:
  - gateways
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gateways
  - httproutes
  - grpcroutes
  - gatewayclasses
  - referencegrants
  verbs:
  - get
  - watch
  - list
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - gatewayclasses/status
  verbs:
  - update
{{- end -}}
