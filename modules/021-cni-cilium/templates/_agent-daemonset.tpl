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
        {{ include "helm_lib_prevent_ds_eviction_annotation" . | nindent 8 }}
      labels:
        app: agent
        module: cni-cilium
    spec:
      {{- include "helm_lib_priority_class" (tuple $context "system-node-critical") | nindent 6 }}
      {{- include "helm_lib_tolerations" (tuple $context "any-node" "with-uninitialized" "with-cloud-provider-uninitialized" "with-storage-problems") | nindent 6 }}
      {{- include "helm_lib_module_pod_security_context_run_as_user_root" $context | nindent 6 }}
      automountServiceAccountToken: true
      imagePullSecrets:
      - name: deckhouse-registry
      containers:
      - name: cilium-agent
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        command:
        - /bin/sh
        - -ec
        - |
          cp -a /var/lib/cilium-rw/bpf /var/lib/cilium/;
          exec cilium-agent --config-dir=/tmp/cilium/config-map \
                            --prometheus-serve-addr=127.0.0.1:9092
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
          initialDelaySeconds: 5
        livenessProbe:
          httpGet:
            host: "127.0.0.1"
            path: /healthz
            port: 9876
            scheme: HTTP
            httpHeaders:
            - name: "brief"
              value: "true"
            - name: "require-k8s-connectivity"
              value: "false"
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
        - name: GOMEMLIMIT
          valueFrom:
            resourceFieldRef:
              resource: limits.memory
              divisor: '1'
        - name: KUBERNETES_SERVICE_HOST
          value: "127.0.0.1"
        - name: KUBERNETES_SERVICE_PORT
          value: "6445"
        lifecycle:
          preStop:
            exec:
              command:
              - /cni-uninstall.sh
        securityContext:
          privileged: false
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
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
              # Needed to switch network namespaces (used for health endpoint, socket-LB).
              # We need it for now but might not need it for >= 5.11 specially
              # for the 'SYS_RESOURCE'.
              # In >= 5.8 there's already BPF and PERMON capabilities
              - SYS_ADMIN
              # Could be an alternative for the SYS_ADMIN for the RLIMIT_NPROC
              - SYS_RESOURCE
              # Both PERFMON and BPF requires kernel 5.8, container runtime
              # cri-o >= v1.22.0 or containerd >= v1.5.0.
              # If available, SYS_ADMIN can be removed.
              - PERFMON
              - BPF
              # Allow discretionary access control (e.g. required for package installation)
              - DAC_OVERRIDE
              # Allow to set Access Control Lists (ACLs) on arbitrary files (e.g. required for package installation)
              - FOWNER
              # Allow to execute program that changes GID (e.g. required for package installation)
              - SETGID
              # Allow to execute program that changes UID (e.g. required for package installation)
              - SETUID
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
        - name: cilium-netns
          mountPath: /var/run/cilium/netns
          mountPropagation: HostToContainer
        - name: cni-path
          mountPath: /host/opt/cni/bin
        - name: etc-cni-netd
          mountPath: /host/etc/cni/net.d
        - name: var-lib-cilium-include-bpf
          mountPath: /var/lib/cilium/bpf
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
        - name: tmp
          mountPath: /tmp
        - name: root-config
          mountPath: /root/.config
        resources:
        {{ include "helm_lib_resources_management_pod_resources" (list $context.Values.cniCilium.resourcesManagement) | nindent 10 }}
      - name: kube-rbac-proxy
        {{- include "helm_lib_module_container_security_context_pss_restricted_flexible" dict | nindent 8 }}
        image: {{ include "helm_lib_module_image" (list $context "kubeRbacProxy") }}
        args:
        - "--secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):4241"
        - "--v=2"
        - "--logtostderr=true"
        - "--stale-cache-interval=1h30m"
        - "--livez-path=/livez"
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
        - containerPort: 4241
          name: https-metrics
        livenessProbe:
          httpGet:
            path: /livez
            port: 4241
            scheme: HTTPS
        readinessProbe:
          httpGet:
            path: /livez
            port: 4241
            scheme: HTTPS
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
  {{- if not ($context.Values.global.enabledModules | has "vertical-pod-autoscaler") }}
            {{- include "helm_lib_container_kube_rbac_proxy_resources" $context | nindent 12 }}
  {{- end }}
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      initContainers:
      - name: check-wg-kernel-compat
        image: {{ include "helm_lib_module_image" (list $context "checkWgKernelCompat") }}
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "NET_ADMIN" "NET_RAW" "SYS_MODULE")) | nindent 8 }}
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
        imagePullPolicy: IfNotPresent
        env:
        - name: WG_KERNEL_CONSTRAINT
          value: ">= 6.8"
        command:
          - "/check-wg-kernel-compat"
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
        - name: cni-path
          mountPath: /hostbin
      - name: check-linux-kernel
        image: {{ include "helm_lib_module_common_image" (list $context "checkKernelVersion") }}
        {{- include "helm_lib_module_container_security_context_run_as_user_deckhouse_pss_restricted" . | nindent 8 }}
          readOnlyRootFilesystem: true
        env:
        - name: KERNEL_CONSTRAINT
          value: "{{ $context.Values.cniCilium.internal.minimalRequiredKernelVersionConstraint }}"
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
      - name: clearing-unnecessary-iptables
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "NET_ADMIN" "NET_RAW" "SYS_MODULE")) | nindent 8 }}
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
        imagePullPolicy: IfNotPresent
        command:
          - "/check-n-cleaning-iptables.sh"
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
        - name: lib-modules
          mountPath: /lib/modules
          readOnly: true
        - name: xtables-lock
          mountPath: /run/xtables.lock
      {{- if eq $context.Values.cniCilium.internal.mode "VXLAN" }}
      - name: handle-vxlan-offload
        image: {{ include "helm_lib_module_common_image" (list $context "vxlanOffloadingFixer") }}
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "NET_ADMIN")) | nindent 8 }}
        imagePullPolicy: IfNotPresent
        env:
        - name: NODE_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
        terminationMessagePolicy: FallbackToLogsOnError
      {{- end }}
      - name: config
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . | nindent 8 }}
        imagePullPolicy: IfNotPresent
        command:
        - cilium-dbg
        - build-config
        - --allow-config-keys=debug,single-cluster-route,mtu,bpf-map-dynamic-size-ratio
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
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
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
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
        resources:
          requests:
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
      - name: apply-sysctl-overwrites
        image: {{ include "helm_lib_module_image" (list $context "agentDistroless") }}
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all_and_add"  (list . (list "SYS_CHROOT" "SYS_ADMIN" "SYS_PTRACE")) | nindent 8 }}
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
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
        securityContext:
          privileged: true
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: true
          capabilities:
            drop:
            - ALL
        args:
        - 'mount | grep "/sys/fs/bpf type bpf" || mount -t bpf bpf /sys/fs/bpf'
        command:
        - /bin/bash
        - -c
        - --
        terminationMessagePolicy: FallbackToLogsOnError
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
        - name: WRITE_CNI_CONF_WHEN_READY
          valueFrom:
            configMapKeyRef:
              name: cilium-config
              key: write-cni-conf-when-ready
              optional: true
        - name: KUBERNETES_SERVICE_HOST
          value: "127.0.0.1"
        - name: KUBERNETES_SERVICE_PORT
          value: "6445"
        securityContext:
          privileged: false
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
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
        {{- include "helm_lib_module_container_security_context_read_only_root_filesystem_capabilities_drop_all" . | nindent 8 }}
          seLinuxOptions:
            level: 's0'
            type: 'spc_t'
        command:
          - "/install-plugin.sh"
        resources:
          requests:
            cpu: 100m
            memory: 10Mi
            {{- include "helm_lib_module_ephemeral_storage_only_logs" $context | nindent 12 }}
        terminationMessagePolicy: FallbackToLogsOnError
        volumeMounts:
          - name: cni-path
            mountPath: /host/opt/cni/bin
      restartPolicy: Always
      serviceAccountName: agent
      terminationGracePeriodSeconds: 1
      volumes:
      - name: tmp
        emptyDir: {}
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
      - name: cilium-netns
        hostPath:
          path: /var/run/netns
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
      - name: root-config
        emptyDir: {}
      - name: var-lib-cilium-include-bpf
        emptyDir: {}
{{- end  }}
