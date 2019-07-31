{{- define "dex-authenticator-config" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  {{- $chart_name := index . 2 }}
  {{- $domain := index . 3 }}
  - id: {{ $chart_name }}-dex-authenticator
    name: {{ $chart_name }}-dex-authenticator
    secret: {{ $config.dexSecret }}
    redirectURIs:
      - https://{{ $domain }}/dex-authenticator/callback
{{- end }}


{{- define "dex-authenticator" }}
  {{- $context := index . 0 }}
  {{- $config := index . 1 }}
  {{- $chart_name := index . 2 }}
  {{- $domain := index . 3 }}
  {{- $use_kubernetes_dex_client_app := index . 4 }}
  {{- $set_authorization_header := index . 5 }}

  {{- $certmanager_cluster_issuer_name := include "certmanager_cluster_issuer_name" $context }}
  {{- $custom_certificate_secret_name := include "custom_certificate_secret_name" $context }}

  {{- if or (and ($certmanager_cluster_issuer_name) ($context.Values.global.enabledModules | has "cert-manager")) ($custom_certificate_secret_name) }}

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
  {{ $chart_name }}: https://{{ $domain }}/dex-authenticator/callback

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
data:
  client-secret: {{ $config.dexSecret | b64enc }}
  cookie-secret: {{ $config.cookieSecret | b64enc }}
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
  replicas: 1
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
      containers:
      - args:
        - --provider=oidc
    {{- if $use_kubernetes_dex_client_app }}
        - --client-id=kubernetes
    {{- else }}
        - --client-id={{ $chart_name }}-dex-authenticator
    {{- end }}
        - --redirect-url=https://{{ $domain }}
        - --oidc-issuer-url=https://{{ include "helm_lib_addon_public_domain" (list $context "dex") }}/
    {{- if $set_authorization_header }}
        - --set-authorization-header=true
    {{- end }}
        - --ssl-insecure-skip-verify=true
        - --proxy-prefix=/dex-authenticator
        - --email-domain=*
        - --upstream=file:///dev/null
        - --http-address=0.0.0.0:4180
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
        image: {{ $context.Values.global.modulesImages.registry }}/user-authn/dex-authenticator:{{ $context.Values.global.modulesImages.tags.userAuthn.dexAuthenticator }}
        name: dex-authenticator
        readinessProbe:
          tcpSocket:
            port: 4180
          initialDelaySeconds: 1
          periodSeconds: 5
        livenessProbe:
          tcpSocket:
            port: 4180
          initialDelaySeconds: 15
          periodSeconds: 10
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
        ports:
        - containerPort: 4180
          protocol: TCP
---
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: {{ $context.Values.global.ingressClass | quote }}
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
          servicePort: 4180
        path: /dex-authenticator
  tls:
  - hosts:
    - {{ $domain }}
    secretName: ingress-tls
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
    port: 4180
  selector:
    app: dex-authenticator
  {{- end }}
{{- end }}
