{{- define "template.nginx" }}
  {{- $name := (print "nginx" (.suffix | default "")) }}
  {{- $publishService := (.publishService | default false) }}
  {{- $hostNetwork := (.hostNetwork | default false) }}
  {{- $updateOnDelete := (.updateOnDelete | default false) }}
  {{- with .context }}
    {{- if (.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
---
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: {{ $name }}
  namespace: {{ include "helper.namespace" . }}
{{ include "helm_lib_module_labels" (list . (dict "app" $name)) | indent 2 }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: DaemonSet
    name: {{ $name }}
  updatePolicy:
    updateMode: "Off"
    {{- end }}
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ $name }}
  namespace: {{ include "helper.namespace" . }}
{{ include "helm_lib_module_labels" (list . (dict "app" $name)) | indent 2 }}
{{- if $updateOnDelete }}
    nginx-ingress-safe-update: ""
{{- end }}
spec:
  updateStrategy:
  {{ if $updateOnDelete }}
    type: OnDelete
  {{ else }}
    type: RollingUpdate
  {{ end }}
  selector:
    matchLabels:
      app: {{ $name }}
  template:
    metadata:
      labels:
        app: {{ $name }}
#TODO: Docker before 1.12 does not support sysctls
#        security.alpha.kubernetes.io/sysctls: "net.ipv4.ip_local_port_range=1024 65000"
{{- if .enableIstioSidecar }}
      annotations:
        sidecar.istio.io/inject: "true"
        traffic.sidecar.istio.io/includeOutboundIPRanges: "{{ .Values.global.discovery.serviceSubnet }}"
{{- end }}
    spec:
{{- include "helm_lib_node_selector" (tuple . "frontend" .) | indent 6 }}
{{- include "helm_lib_tolerations" (tuple . "frontend" .) | indent 6 }}
{{- include "helm_lib_priority_class" (tuple . "cluster-high") | indent 6 }}
      serviceAccount: kube-nginx-ingress
      hostNetwork: {{ $hostNetwork }}
    {{- if eq $hostNetwork true }}
      dnsPolicy: ClusterFirstWithHostNet
    {{- else }}
      dnsPolicy: ClusterFirst
    {{- end }}
      terminationGracePeriodSeconds: 300
      imagePullSecrets:
      - name: deckhouse-registry
      containers:
      - image: {{ .Values.global.modulesImages.registry }}/nginx-ingress/controller:{{ .Values.global.modulesImages.tags.nginxIngress.controller }}
        name: nginx
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        livenessProbe:
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          initialDelaySeconds: 30
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /healthz
            port: 10254
            scheme: HTTP
          periodSeconds: 2
          timeoutSeconds: 5
        args:
        - /nginx-ingress-controller
        {{- if and (hasKey . "customErrorsNamespace") (hasKey . "customErrorsServiceName") (.customErrorsNamespace) (.customErrorsServiceName) }}
        - --default-backend-service={{ .customErrorsNamespace }}/{{ .customErrorsServiceName }}
        {{- end }}
        - --configmap=$(POD_NAMESPACE)/{{ $name }}
    {{- if $publishService }}
        - --publish-service=$(POD_NAMESPACE)/{{ $name }}
    {{- end }}
        - --v=2
    {{- if not .name }}
        - --ingress-class=nginx{{ if .Values.nginxIngress.rewriteTargetMigration }}-rwr{{ end }}
    {{- else }}
        - --ingress-class=nginx-{{ .name }}{{ if .Values.nginxIngress.rewriteTargetMigration }}-rwr{{ end }}
    {{- end }}
        securityContext:
          capabilities:
            drop:
            - ALL
            add:
            - NET_BIND_SERVICE
          runAsUser: 33
        ports:
        - name: controller
          containerPort: 10254
        volumeMounts:
        - mountPath: /var/lib/nginx/body
          name: client-body-temp-path
        - mountPath: /var/lib/nginx/fastcgi
          name: fastcgi-temp-path
        - mountPath: /var/lib/nginx/proxy
          name: proxy-temp-path
        - mountPath: /var/lib/nginx/scgi
          name: scgi-temp-path
        - mountPath: /var/lib/nginx/uwsgi
          name: uwsgi-temp-path
        - mountPath: /etc/nginx/ssl/client.crt
          name: secret-nginx-auth-tls-crt
          subPath: client.crt
          readOnly: true
        - mountPath: /etc/nginx/ssl/client.key
          name: secret-nginx-auth-tls-key
          subPath: client.key
          readOnly: true
      - image: {{ .Values.global.modulesImages.registry }}/nginx-ingress/statsd-exporter:{{ .Values.global.modulesImages.tags.nginxIngress.statsdExporter }}
        name: statsd-exporter
      - name: kube-rbac-proxy
        image: {{ .Values.global.modulesImages.registry }}/common/kube-rbac-proxy:{{ .Values.global.modulesImages.tags.common.kubeRbacProxy }}
        args:
        - "--secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9103"
        - "--client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
        - "--v=2"
        - "--logtostderr=true"
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 9103
          name: https-metrics
        env:
        - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: KUBE_RBAC_PROXY_CONFIG
          value: |
            upstreams:
            - upstream: http://127.0.0.1:10254/metrics
              path: /controller/metrics
              authorization:
                resourceAttributes:
                  namespace: {{ include "helper.namespace" . }}
                  apiGroup: apps
                  apiVersion: v1
                  resource: daemonsets
                  subresource: prometheus-controller-metrics
                  name: nginx-ingress
            - upstream: http://127.0.0.1:9102/metrics
              path: /statsd/metrics
              authorization:
                resourceAttributes:
                  namespace: {{ include "helper.namespace" . }}
                  apiGroup: apps
                  apiVersion: v1
                  resource: daemonsets
                  subresource: prometheus-statsd-metrics
                  name: nginx-ingress
      volumes:
      - name: client-body-temp-path
        emptyDir: {}
      - name: fastcgi-temp-path
        emptyDir: {}
      - name: proxy-temp-path
        emptyDir: {}
      - name: scgi-temp-path
        emptyDir: {}
      - name: uwsgi-temp-path
        emptyDir: {}
      - name: secret-nginx-auth-tls-crt
        secret:
          secretName: nginx-auth-tls
          items:
          - key: tls.crt
            path: client.crt
      - name: secret-nginx-auth-tls-key
        secret:
          secretName: nginx-auth-tls
          items:
          - key: tls.key
            path: client.key
  {{- end }}
{{- end }}
