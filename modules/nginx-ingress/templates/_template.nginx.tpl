{{- define "template.nginx" }}
  {{- $name := (print "nginx" (.suffix | default "")) }}
  {{- $publishService := (.publishService | default false) }}
  {{- $hostNetwork := (.hostNetwork | default false) }}
  {{- with .context }}
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
      annotations:
        checksum/config-template: {{ .Files.Get "files/nginx.tmpl" | sha256sum }}
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
      containers:
      - image: quay.io/kubernetes-ingress-controller/nginx-ingress-controller:0.11.0
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
        - --default-backend-service=$(POD_NAMESPACE)/default-http-backend
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
        volumeMounts:
        - mountPath: /etc/nginx/template
          name: nginx-config-template
      volumes:
      - name: nginx-config-template
        configMap:
          name: nginx-config-template
  {{- end }}
{{- end }}
