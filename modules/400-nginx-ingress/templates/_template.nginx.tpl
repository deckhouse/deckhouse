{{- define "template.nginx" }}
  {{- $name := (print "nginx" (.suffix | default "")) }}
  {{- $publishService := (.publishService | default false) }}
  {{- $hostNetwork := (.hostNetwork | default false) }}
  {{- with .context }}
{{- if semverCompare ">=1.11" .Values.global.discovery.clusterVersion }}
---
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: {{ $name }}
  namespace: {{ include "helper.namespace" . }}
  labels:
    heritage: antiopa
    module: {{ .Chart.Name }}
    app: {{ $name }}
spec:
  targetRef:
    apiVersion: "extensions/v1beta1"
    kind: DaemonSet
    name: {{ $name }}
  updatePolicy:
    updateMode: "Off"
{{- end }}
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: {{ $name }}
  namespace: {{ include "helper.namespace" . }}
  labels:
    heritage: antiopa
    module: {{ .Chart.Name }}
    app: {{ $name }}
spec:
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app: {{ $name }}
  template:
    metadata:
      labels:
        app: {{ $name }}
#TODO: Docker before 1.12 does not support sysctls
#        security.alpha.kubernetes.io/sysctls: "net.ipv4.ip_local_port_range=1024 65000"
    spec:
{{ include "helper.nodeSelector" . | indent 6 }}
{{ include "helper.tolerations" . | indent 6 }}
      serviceAccount: kube-nginx-ingress
      hostNetwork: {{ $hostNetwork }}
    {{- if eq $hostNetwork true }}
      dnsPolicy: ClusterFirstWithHostNet
    {{- else }}
      dnsPolicy: ClusterFirst
    {{- end }}
      terminationGracePeriodSeconds: 300
      imagePullSecrets:
      - name: registry
      {{- if semverCompare ">=1.11" .Values.global.discovery.clusterVersion }}
      priorityClassName: cluster-high
      {{- end }}
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
        {{- else }}
        - --default-backend-service=$(POD_NAMESPACE)/default-http-backend
        {{- end }}
        - --configmap=$(POD_NAMESPACE)/{{ $name }}
        - --annotations-prefix=ingress.kubernetes.io
    {{- if $publishService }}
        - --publish-service=$(POD_NAMESPACE)/{{ $name }}
    {{- end }}
        - --sort-backends
        - --v=2
    {{- if not .name }}
        - --ingress-class=nginx
    {{- else }}
        - --ingress-class=nginx-{{ .name }}
    {{- end }}
        securityContext:
          capabilities:
            drop:
            - ALL
            add:
            - NET_BIND_SERVICE
          runAsUser: 33
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
      - image: {{ .Values.global.modulesImages.registry }}/nginx-ingress/statsd-exporter:{{ .Values.global.modulesImages.tags.nginxIngress.statsdExporter }}
        name: statsd-exporter
      - name: prometheus-auth-proxy
        image: flant/kube-prometheus-auth-proxy:v0.1.0
        args:
        - "--listen=$(MY_POD_IP):9103"
        - "--proxy-pass=http://127.0.0.1:9102/metrics"
        env:
        - name: MY_POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        ports:
        - containerPort: 9103
        resources:
          requests:
            memory: 20Mi
            cpu: 10m
          limits:
            memory: 40Mi
            cpu: 20m
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
  {{- end }}
{{- end }}
