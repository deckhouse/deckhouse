{{- define "dex-authenticator-config" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  {{- $chart_name := index . 2 }}
  {{- $domain := index . 3 }}
  - id: {{ $chart_name }}-dex-authenticator
    name: {{ $chart_name }}-dex-authenticator
    secret: {{ $config.dexSecret }}
    redirectURIs:
      - {{ include "helm_lib_module_uri_scheme" $context }}://{{ $domain }}/dex-authenticator/callback
{{- end }}


{{- define "dex-authenticator" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  {{- $chart_name := index . 2 }}
  {{- $domain := index . 3 }}
  {{- $use_kubernetes_dex_client_app := index . 4 }}
  {{- $set_authorization_header := index . 5 }}

  {{- if $use_kubernetes_dex_client_app }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ $chart_name}}-kubernetes-dex-client-app-redirect-uris
  namespace: kube-user-authn
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
data:
  {{ $chart_name }}: {{ include "helm_lib_module_uri_scheme" $context }}://{{ $domain }}/dex-authenticator/callback

  {{- else }}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $chart_name }}-dex-client-app
  namespace: kube-user-authn
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
data:
  config.yaml: |
    {{ include "dex-authenticator-config" (list $context $config $chart_name $domain) | b64enc }}
  {{- end }}
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator
  namespace: kube-{{ $chart_name }}
  app: dex-authenticator
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
data:
  client-secret: {{ $config.dexSecret | b64enc }}
  cookie-secret: {{ $config.cookieSecret | b64enc }}
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-tls
  namespace: kube-{{ $chart_name }}
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
type: kubernetes.io/tls
data:
  tls.crt: {{ $config.pem | b64enc }}
  tls.key: {{ $config.key | b64enc }}
  ca.crt: {{ $config.ca | b64enc }}

  {{- if semverCompare ">=1.11" $context.Values.global.discovery.clusterVersion }}
---
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: dex-authenticator
  namespace: kube-{{ $chart_name }}
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: dex-authenticator
  updatePolicy:
    updateMode: "Auto"
  {{- end }}
---
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  annotations:
    helm.sh/hook: post-upgrade, post-install
    helm.sh/hook-delete-policy: before-hook-creation
  name: dex-authenticator
  namespace: kube-{{ $chart_name }}
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
spec:
  minAvailable: {{ include "helm_lib_is_ha_to_value" (list $context 1 0 ) }}
  selector:
    matchLabels:
      app: dex-authenticator
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dex-authenticator
  namespace: kube-{{ $chart_name }}
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
spec:
  replicas: {{ include "helm_lib_is_ha_to_value" (list $context 2 1) }}
  selector:
    matchLabels:
      app: dex-authenticator
  template:
    metadata:
      labels:
        app: dex-authenticator
    spec:
  {{- include "helm_lib_node_selector" (tuple $context "system") | indent 6 }}
  {{- include "helm_lib_tolerations" (tuple $context "system") | indent 6 }}
      imagePullSecrets:
      - name: antiopa-registry
    {{- if semverCompare ">=1.11" $context.Values.global.discovery.clusterVersion }}
      priorityClassName: cluster-low
    {{- end }}
      volumes:
      - name: tls
        secret:
          secretName: dex-authenticator-tls
      containers:
      - args:
        - --provider=oidc
    {{- if $use_kubernetes_dex_client_app }}
        - --client-id=kubernetes
    {{- else }}
        - --client-id={{ $chart_name }}-dex-authenticator
    {{- end }}
    {{- if ne (include "helm_lib_module_uri_scheme" $context) "https" }}
        - --cookie-secure=false
    {{- end }}
        - --redirect-url={{ include "helm_lib_module_uri_scheme" $context }}://{{ $domain }}
        - --oidc-issuer-url=https://{{ include "helm_lib_module_public_domain" (list $context "dex") }}/
    {{- if $set_authorization_header }}
        - --set-authorization-header=true
    {{- end }}
        - --scope=groups email openid
        - --ssl-insecure-skip-verify=true
        - --proxy-prefix=/dex-authenticator
        - --email-domain=*
        - --upstream=file:///dev/null
        - --tls-cert=/opt/dex-authenticator/tls/tls.crt
        - --tls-key=/opt/dex-authenticator/tls/tls.key
        env:
        - name: OAUTH2_PROXY_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: dex-authenticator
              key: client-secret
        - name: OAUTH2_PROXY_COOKIE_SECRET
          valueFrom:
            secretKeyRef:
              name: dex-authenticator
              key: cookie-secret
        volumeMounts:
        - name: tls
          mountPath: "/opt/dex-authenticator/tls"
          readOnly: true
        image: {{ $context.Values.global.modulesImages.registry }}/user-authn/dex-authenticator:{{ $context.Values.global.modulesImages.tags.userAuthn.dexAuthenticator }}
        name: dex-authenticator
        readinessProbe:
          tcpSocket:
            port: 443
            scheme: HTTPS
          initialDelaySeconds: 1
          periodSeconds: 5
        livenessProbe:
          tcpSocket:
            port: 443
            scheme: HTTPS
          initialDelaySeconds: 15
          periodSeconds: 10
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
        ports:
        - containerPort: 443
          protocol: TCP
---
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: {{ include "helm_lib_module_ingress_class" $context | quote }}
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/proxy-buffer-size: "128k"
  name: dex-authenticator
  namespace: kube-{{ $chart_name }}
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
spec:
  rules:
  - host: {{ $domain }}
    http:
      paths:
      - backend:
          serviceName: dex-authenticator
          servicePort: 443
        path: /dex-authenticator
  {{- if (include "helm_lib_module_https_ingress_tls_enabled" $context) }}
  tls:
  - hosts:
    - {{ $domain }}
    secretName: ingress-tls
  {{- end }}
---
apiVersion: v1
kind: Service
metadata:
  labels:
    heritage: antiopa
    module: {{ $chart_name }}
    app: dex-authenticator
  name: dex-authenticator
  namespace: kube-{{ $chart_name }}
spec:
  ports:
  - name: http
    port: 443
  selector:
    app: dex-authenticator
{{- end }}
