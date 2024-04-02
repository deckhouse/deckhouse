{{- define "agent_daemonset_template" }}
  {{- $context := index . 0 }}
  {{- $agent_daemonset_generation := index . 1 }}
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: agent
  namespace: d8-{{ $context.Chart.Name }}
  {{ include "helm_lib_module_labels" (list $context (dict "app" "agent")) | nindent 2 }}
spec:
  selector:
    matchLabels:
      app: agent
  updateStrategy:
    type: OnDelete
  template:
    metadata:
      annotations:
        configmap-checksum: {{ include (print $context.Template.BasePath "/configmap.yaml") $context | sha256sum | quote }}
        safe-agent-updater-daemonset-generation: {{ $agent_daemonset_generation | quote }}
        container.apparmor.security.beta.kubernetes.io/cilium-agent: "unconfined"
        container.apparmor.security.beta.kubernetes.io/clean-cilium-state: "unconfined"
        container.apparmor.security.beta.kubernetes.io/mount-cgroup: "unconfined"
        container.apparmor.security.beta.kubernetes.io/apply-sysctl-overwrites: "unconfined"
        container.apparmor.security.beta.kubernetes.io/install-cni-binaries: "unconfined"
      labels:
        app: agent
        module: cni-cilium
    spec:
      {{- include "helm_lib_priority_class" (tuple $context "system-node-critical") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node" "with-uninitialized" "with-cloud-provider-uninitialized" "with-storage-problems") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_root" $context | nindent 6 }}
      imagePullSecrets:
      - name: deckhouse-registry
      containers:
      - name: cilium-agent
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        command:
        - cilium-agent
        args:
        - --config-dir=/tmp/cilium/config-map
        startupProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: 9876
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
          failureThreshold: 105
          periodSeconds: 2
          successThreshold: 1
        livenessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: 9876
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
          periodSeconds: 30
          successThreshold: 1
          failureThreshold: 10
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: 9876
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
          periodSeconds: 30
          successThreshold: 1
          failureThreshold: 3
          timeoutSeconds: 5
        env:
        - name: K8S_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: CILIUM_K8S_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: KUBERNETES_SERVICE_HOST
          value: "127.0.0.1"
        - name: KUBERNETES_SERVICE_PORT
          value: "6445"
        lifecycle:
          preStop:
            exec:
              command:
              - /cni-uninstall.sh
        ports:
        - name: prometheus
          containerPort: 9092
          hostPort: 9092
          protocol: TCP
        securityContext:
          privileged: false
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
          capabilities:
            add:
              # Use to set socket permission
              - CHOWN
              # Used to terminate envoy child process
              - KILL
              # Used since cilium modifies routing tables, etc...
              - NET_ADMIN
              # Used since cilium creates raw sockets, etc...
              - NET_RAW
              # Used since cilium monitor uses mmap
              - IPC_LOCK
              # Used in iptables. Consider removing once we are iptables-free
              - SYS_MODULE
              # We need it for now but might not need it for >= 5.11 specially
              # for the 'SYS_RESOURCE'.
              # In >= 5.8 there's already BPF and PERMON capabilities
              - SYS_ADMIN
              # Could be an alternative for the SYS_ADMIN for the RLIMIT_NPROC
              - SYS_RESOURCE
              # Both PERFMON and BPF requires kernel 5.8, container runtime
              # cri-o >= v1.22.0 or containerd >= v1.5.0.
              # If available, SYS_ADMIN can be removed.
              #- PERFMON
              #- BPF
              # Allow discretionary access control (e.g. required for package installation)
              - DAC_OVERRIDE
            drop:
              - ALL
        volumeMounts:
        - mountPath: /sys/fs/bpf
          mountPropagation: HostToContainer
          name: bpf-maps
        - mountPath: /host/proc/sys/net
          name: host-proc-sys-net
        - mountPath: /host/proc/sys/kernel
          name: host-proc-sys-kernel
        - name: cilium-cgroup
          mountPath: "/run/cilium/cgroupv2"
        - name: cilium-run
          mountPath: /var/run/cilium
        - name: cni-path
          mountPath: /host/opt/cni/bin
        - name: etc-cni-netd
          mountPath: /host/etc/cni/net.d
        - name: cilium-config-path
          mountPath: /tmp/cilium/config-map
          readOnly: true
        {{- if has "virtualization" $context.Values.global.enabledModules }}
        - mountPath: /etc/config
          name: ip-masq-agent
          readOnly: true
        {{- end }}
          # Needed to be able to load kernel modules
        - name: lib-modules
          mountPath: /lib/modules
          readOnly: true
        - name: xtables-lock
          mountPath: /run/xtables.lock
        - name: hubble-tls
          mountPath: /var/lib/cilium/tls/hubble
          readOnly: true
        resources:
        {{ include "helm_lib_resources_management_pod_resources" (list $context.Values.cniCilium.resourcesManagement) | nindent 10 }}
      - name: kube-rbac-proxy
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem" $context | nindent 8 }}
        image: {{ include "helm_lib_module_image" (list $context "kubeRbacProxy") }}
        args:
        - "--secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):9734"
        - "--v=2"
        - "--logtostderr=true"
        - "--stale-cache-interval=1h30m"
        env:
        - name: KUBE_RBAC_PROXY_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: KUBE_RBAC_PROXY_CONFIG
          value: |
            upstreams:
            - upstream: http://127.0.0.1:9092/metrics
              path: /metrics
              authorization:
                resourceAttributes:
                  namespace: d8-{{ $context.Chart.Name }}
                  apiGroup: apps
                  apiVersion: v1
                  resource: daemonsets
                  subresource: prometheus-metrics
                  name: agent
        ports:
        - containerPort: 9734
          name: https-metrics
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
  {{- if not ($context.Values.global.enabledModules | has "vertical-pod-autoscaler-crd") }}
            {{- include "helm_lib_container_kube_rbac_proxy_resources" $context | nindent 12 }}
  {{- end }}
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      initContainers:
      {{- include "module_init_container_check_linux_kernel" (tuple $context ">= 4.9.17") | nindent 6 }}
      - name: mount-cgroup
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        env:
        - name: CGROUP_ROOT
          value: "/run/cilium/cgroupv2"
        - name: BIN_PATH
          value: "/opt/cni/bin"
        command:
        - sh
        - -ec
        - |
          cp /usr/bin/cilium-mount /hostbin/cilium-mount;
          nsenter --cgroup=/hostproc/1/ns/cgroup --mount=/hostproc/1/ns/mnt "${BIN_PATH}/cilium-mount" "$CGROUP_ROOT";
          rm /hostbin/cilium-mount
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
        - name: hostproc
          mountPath: /hostproc
        - name: cni-path
          mountPath: /hostbin
        securityContext:
          privileged: false
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
          capabilities:
            drop:
              - ALL
            add:
              # Only used for 'mount' cgroup
              - SYS_ADMIN
              # Used for nsenter
              - SYS_CHROOT
              - SYS_PTRACE
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
      - name: apply-sysctl-overwrites
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        env:
        - name: BIN_PATH
          value: /opt/cni/bin
        command:
        - sh
        - -ec
        - |
          cp /usr/bin/cilium-sysctlfix /hostbin/cilium-sysctlfix;
          nsenter --mount=/hostproc/1/ns/mnt "${BIN_PATH}/cilium-sysctlfix";
          rm /hostbin/cilium-sysctlfix
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          privileged: false
          seLinuxOptions:
            level: s0
            type: spc_t
          capabilities:
            add:
              - SYS_ADMIN
              - SYS_CHROOT
              - SYS_PTRACE
            drop:
              - ALL
        volumeMounts:
          - name: hostproc
            mountPath: /hostproc
          - name: cni-path
            mountPath: /hostbin
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
      - name: mount-bpf-fs
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        args:
        - 'mount | grep "/sys/fs/bpf type bpf" || mount -t bpf bpf /sys/fs/bpf'
        command:
        - /bin/bash
        - -c
        - --
        terminationMessagePolicy: FallbackToLogsOnError
        securityContext:
          privileged: true
        volumeMounts:
        - name: bpf-maps
          mountPath: /sys/fs/bpf
          mountPropagation: Bidirectional
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
      - name: clean-cilium-state
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        command:
        - /init-container.sh
        env:
        - name: CILIUM_ALL_STATE
          valueFrom:
            configMapKeyRef:
              name: cilium-config
              key: clean-cilium-state
              optional: true
        - name: CILIUM_BPF_STATE
          valueFrom:
            configMapKeyRef:
              name: cilium-config
              key: clean-cilium-bpf-state
              optional: true
        - name: KUBERNETES_SERVICE_HOST
          value: "127.0.0.1"
        - name: KUBERNETES_SERVICE_PORT
          value: "6445"
        securityContext:
          privileged: false
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
          capabilities:
            # Most of the capabilities here are the same ones used in the
            # cilium-agent's container because this container can be used to
            # uninstall all Cilium resources, and therefore it is likely that
            # will need the same capabilities.
            add:
              # Used since cilium modifies routing tables, etc...
              - NET_ADMIN
              # Used in iptables. Consider removing once we are iptables-free
              - SYS_MODULE
              # We need it for now but might not need it for >= 5.11 specially
              # for the 'SYS_RESOURCE'.
              # In >= 5.8 there's already BPF and PERMON capabilities
              - SYS_ADMIN
              # Could be an alternative for the SYS_ADMIN for the RLIMIT_NPROC
              - SYS_RESOURCE
              # Both PERFMON and BPF requires kernel 5.8, container runtime
              # cri-o >= v1.22.0 or containerd >= v1.5.0.
              # If available, SYS_ADMIN can be removed.
              #- PERFMON
              #- BPF
            drop:
              - ALL
        volumeMounts:
        - name: bpf-maps
          mountPath: /sys/fs/bpf
        - name: cilium-cgroup
          mountPath: /run/cilium/cgroupv2
          mountPropagation: HostToContainer
        - name: cilium-run
          mountPath: /var/run/cilium
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
      # Install the CNI binaries in an InitContainer so we don't have a writable host mount in the agent
      - name: install-cni-binaries
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        command:
          - "/install-plugin.sh"
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
        securityContext:
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
          capabilities:
            drop:
              - ALL
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
          - name: cni-path
            mountPath: /host/opt/cni/bin
      restartPolicy: Always
      serviceAccountName: agent
      terminationGracePeriodSeconds: 1
      volumes:
      - name: host-proc-sys-net
        hostPath:
          type: Directory
          path: /proc/sys/net
      - name: host-proc-sys-kernel
        hostPath:
          type: Directory
          path: /proc/sys/kernel
      - name: cilium-run
        hostPath:
          path: "/var/run/cilium"
          type: DirectoryOrCreate
      - name: bpf-maps
        hostPath:
          path: /sys/fs/bpf
          type: DirectoryOrCreate
      - name: hostproc
        hostPath:
          path: /proc
          type: Directory
      - name: cilium-cgroup
        hostPath:
          path: "/run/cilium/cgroupv2"
          type: DirectoryOrCreate
      - name: cni-path
        hostPath:
          path:  "/opt/cni/bin"
          type: DirectoryOrCreate
      - name: etc-cni-netd
        hostPath:
          path: "/etc/cni/net.d"
          type: DirectoryOrCreate
      - name: lib-modules
        hostPath:
          path: /lib/modules
      - name: xtables-lock
        hostPath:
          path: /run/xtables.lock
          type: FileOrCreate
      - name: cilium-config-path
        configMap:
          name: cilium-config
      {{- if has "virtualization" $context.Values.global.enabledModules }}
      - name: ip-masq-agent
        configMap:
          name: ip-masq-agent
          optional: true
          items:
          - key: config
            path: ip-masq-agent
      {{- end }}
      - name: hubble-tls
        projected:
          defaultMode: 0400
          sources:
          - secret:
              name: hubble-server-certs
              optional: true
              items:
              - key: ca.crt
                path: client-ca.crt
              - key: tls.crt
                path: server.crt
              - key: tls.key
                path: server.key
{{- end  }}
