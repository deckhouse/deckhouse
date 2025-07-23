# Copyright 2024 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
_on_containerd_config_changed() {
  bb-flag-set containerd-need-restart
}


migrate() {
  bb-log-info "start containerd migration"
  systemctl stop kubelet.service
  bb-flag-set kubelet-need-restart
  crictl ps -q | xargs -r crictl stop -t 0 && crictl ps -a -q | xargs -r crictl rm -f
  systemctl stop containerd-deckhouse.service
  for i in $(mount | grep /var/lib/containerd | cut -d " " -f3); do umount $i; done
  if [ -d /var/lib/containerd/io.containerd.snapshotter.v1.erofs ]; then
    chattr -i /var/lib/containerd/io.containerd.snapshotter.v1.erofs/snapshots/*/layer.erofs
  fi
  rm -rf /var/lib/containerd/*
  bb-flag-set containerd-need-restart
  bb-flag-set need-local-images-import
  bb-flag-set reboot
  bb-flag-unset cntrd-major-version-changed
  bb-flag-unset disruption
  bb-log-info "finish containerd migration"
}

bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

  {{- $max_concurrent_downloads := 3 }}
  {{- if hasKey .nodeGroup.cri "containerd" }}
    {{- $max_concurrent_downloads = .nodeGroup.cri.containerd.maxConcurrentDownloads | default $max_concurrent_downloads }}
  {{- end }}
  
  {{- $sandbox_image := "registry.k8s.io/pause:3.2" }}
  {{- if (((.images).registrypackages).pause) }}
    {{ $sandbox_image = "deckhouse.local/images:pause" }}
  {{- end }}

  {{- $default_runtime := "runc" }}
  {{- if .nodeGroup.gpu }}
    {{ $default_runtime = "nvidia" }}
sed -i "s/net.core.bpf_jit_harden = 2/net.core.bpf_jit_harden = 1/" /etc/sysctl.d/99-sysctl.conf # https://github.com/NVIDIA/nvidia-container-toolkit/issues/117#issuecomment-1758781872
sed -i "s/net.core.bpf_jit_harden = 2/net.core.bpf_jit_harden = 1/" /etc/sysctl.conf # REDOS 
  {{- end }}

systemd_cgroup=true
# Overriding cgroup type from external config file
if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
  systemd_cgroup=false
fi


{{- if eq .cri "ContainerdV2" }}
# generated using `containerd config migrate` by containerd version `containerd containerd.io 2.0.4 1a43cb6a1035441f9aca8f5666a9b3ef9e70ab20`
bb-sync-file /etc/containerd/deckhouse.toml - << EOF
version = 3
root = "/var/lib/containerd"
state = "/run/containerd"
plugin_dir = ""
disabled_plugins = []
required_plugins = []
oom_score = 0

[grpc]
  address = "/run/containerd/containerd.sock"
  tcp_address = ""
  tcp_tls_cert = ""
  tcp_tls_key = ""
  uid = 0
  gid = 0
  max_recv_message_size = 16777216
  max_send_message_size = 16777216

[ttrpc]
  address = ""
  uid = 0
  gid = 0

[debug]
  address = ""
  uid = 0
  gid = 0
  level = ""

[metrics]
  address = ""
  grpc_histogram = false

[plugins]
  [plugins.'io.containerd.cri.v1.images']
    snapshotter = "erofs"
    disable_snapshot_annotations = true
    discard_unpacked_layers = true
    max_concurrent_downloads = {{ $max_concurrent_downloads }}
    image_pull_with_sync_fs = false
    image_pull_progress_timeout = '5m0s'
    stats_collect_period = 10

    [plugins.'io.containerd.cri.v1.images'.pinned_images]
      sandbox = {{ $sandbox_image | quote }}

    [plugins.'io.containerd.cri.v1.images'.registry]
      config_path = "/etc/containerd/registry.d"

    [plugins.'io.containerd.cri.v1.images'.image_decryption]
      key_model = ''    
    
  [plugins.'io.containerd.cri.v1.runtime']
    enable_selinux = false
    selinux_category_range = 1024
    max_container_log_line_size = 16384
    disable_apparmor = false
    restrict_oom_score_adj = false
    disable_proc_mount = false
    unset_seccomp_profile = ""
    tolerate_missing_hugetlb_controller = true
    disable_hugetlb_controller = true
    device_ownership_from_security_context = true
    ignore_image_defined_volumes = false
    netns_mounts_under_state_dir = false
    enable_unprivileged_ports = true
    enable_unprivileged_icmp = true
    enable_cdi = true
    cdi_spec_dirs = ['/etc/cdi', '/var/run/cdi']
    drain_exec_sync_io_timeout = '0s'
    ignore_deprecation_warnings = []
    [plugins.'io.containerd.cri.v1.runtime'.containerd]
      default_runtime_name = {{ $default_runtime | quote }}
      ignore_blockio_not_enabled_errors = false
      ignore_rdt_not_enabled_errors = false
  {{- if .nodeGroup.gpu }}
      [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes]
        [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.nvidia]
          privileged_without_host_devices = false
          runtime_engine = ""
          runtime_root = ""
          runtime_type = "io.containerd.runc.v2"
          [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.nvidia.options]
            BinaryName = "/usr/bin/nvidia-container-runtime"
            SystemdCgroup = ${systemd_cgroup}
  {{ end }}
        [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc]
          runtime_type = 'io.containerd.runc.v2'
          runtime_path = ''
          pod_annotations = []
          container_annotations = []
          privileged_without_host_devices = false
          privileged_without_host_devices_all_devices_allowed = false
          base_runtime_spec = ''
          cni_conf_dir = ''
          cni_max_conf_num = 0
          sandboxer = 'podsandbox'
          io_type = ''

          [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc.options]
            BinaryName = ''
            CriuImagePath = ''
            CriuWorkPath = ''
            IoGid = 0
            IoUid = 0
            NoNewKeyring = false
            Root = ''
            ShimCgroup = ''
            SystemdCgroup = ${systemd_cgroup}
    [plugins.'io.containerd.cri.v1.runtime'.cni]
      bin_dirs = ['/opt/cni/bin']
      conf_dir = '/etc/cni/net.d'
      max_conf_num = 1
      setup_serially = false
      conf_template = ''
      ip_pref = ''
      use_internal_loopback = false

  [plugins.'io.containerd.differ.v1.erofs']
    mkfs_options = []

  [plugins.'io.containerd.gc.v1.scheduler']
    pause_threshold = 0.02
    deletion_threshold = 0
    mutation_threshold = 100
    schedule_delay = '0s'
    startup_delay = '100ms'

  [plugins."io.containerd.grpc.v1.cri"]
    disable_tcp_service = true
    stream_server_address = "127.0.0.1"
    stream_server_port = "0"
    stream_idle_timeout = "4h0m0s"
    enable_tls_streaming = false

    [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
      tls_cert_file = ""
      tls_key_file = ""

  [plugins."io.containerd.internal.v1.opt"]
    path = "/opt/containerd"

  [plugins.'io.containerd.metadata.v1.bolt']
    content_sharing_policy = 'shared'
    no_sync = false

  [plugins.'io.containerd.monitor.container.v1.restart']
    interval = "10s"
    
  [plugins.'io.containerd.monitor.task.v1.cgroups']
    no_prometheus = false

  [plugins."io.containerd.runtime.v2.task"]
    platforms = ["linux/amd64"]

  [plugins.'io.containerd.service.v1.diff-service']
    default = ['erofs']
    sync_fs = false

  [plugins.'io.containerd.service.v1.tasks-service']
    blockio_config_file = ''
    rdt_config_file = ''

  [plugins.'io.containerd.shim.v1.manager']
    env = []

  [plugins.'io.containerd.snapshotter.v1.erofs']
    root_path = ''
    ovl_mount_options = []

  [plugins.'io.containerd.transfer.v1.local']
    max_concurrent_downloads = {{ $max_concurrent_downloads }}
    concurrent_layer_fetch_buffer = 0
    max_concurrent_uploaded_layers = 3
    check_platform_supported = false
    config_path = ''
    
[cgroup]
  path = ""

[timeouts]
  'io.containerd.timeout.bolt.open' = '0s'
  'io.containerd.timeout.cri.defercleanup' = '1m0s'
  'io.containerd.timeout.metrics.shimstats' = '2s'
  "io.containerd.timeout.shim.cleanup" = "5s"
  "io.containerd.timeout.shim.load" = "5s"
  "io.containerd.timeout.shim.shutdown" = "3s"
  "io.containerd.timeout.task.state" = "2s"
EOF

{{- end }}

{{- if eq .cri "Containerd" }}
# generated using `containerd config default` by containerd version `containerd containerd.io 1.4.3 269548fa27e0089a8b8278fc4fc781d7f65a939b`
bb-sync-file /etc/containerd/deckhouse.toml - << EOF
version = 2
root = "/var/lib/containerd"
state = "/run/containerd"
plugin_dir = ""
disabled_plugins = []
required_plugins = []
oom_score = 0
[grpc]
  address = "/run/containerd/containerd.sock"
  tcp_address = ""
  tcp_tls_cert = ""
  tcp_tls_key = ""
  uid = 0
  gid = 0
  max_recv_message_size = 16777216
  max_send_message_size = 16777216
[ttrpc]
  address = ""
  uid = 0
  gid = 0
[debug]
  address = ""
  uid = 0
  gid = 0
  level = ""
[metrics]
  address = ""
  grpc_histogram = false
[cgroup]
  path = ""
[timeouts]
  "io.containerd.timeout.shim.cleanup" = "5s"
  "io.containerd.timeout.shim.load" = "5s"
  "io.containerd.timeout.shim.shutdown" = "3s"
  "io.containerd.timeout.task.state" = "2s"
[plugins]
  [plugins."io.containerd.gc.v1.scheduler"]
    pause_threshold = 0.02
    deletion_threshold = 0
    mutation_threshold = 100
    schedule_delay = "0s"
    startup_delay = "100ms"
  [plugins."io.containerd.grpc.v1.cri"]
    disable_tcp_service = true
    stream_server_address = "127.0.0.1"
    stream_server_port = "0"
    stream_idle_timeout = "4h0m0s"
    enable_selinux = false
    selinux_category_range = 1024
    sandbox_image = {{ $sandbox_image | quote }}
    stats_collect_period = 10
    systemd_cgroup = false
    enable_tls_streaming = false
    max_container_log_line_size = 16384
    disable_cgroup = false
    disable_apparmor = false
    restrict_oom_score_adj = false
    max_concurrent_downloads = {{ $max_concurrent_downloads }}
    disable_proc_mount = false
    unset_seccomp_profile = ""
    tolerate_missing_hugetlb_controller = true
    disable_hugetlb_controller = true
    ignore_image_defined_volumes = false
    device_ownership_from_security_context = true
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "overlayfs"
      default_runtime_name = {{ $default_runtime | quote }}
      no_pivot = false
      disable_snapshot_annotations = true
      discard_unpacked_layers = true
      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        runtime_type = ""
        runtime_engine = ""
        runtime_root = ""
        privileged_without_host_devices = false
        base_runtime_spec = ""
      [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
        runtime_type = ""
        runtime_engine = ""
        runtime_root = ""
        privileged_without_host_devices = false
        base_runtime_spec = ""
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
          runtime_engine = ""
          runtime_root = ""
          privileged_without_host_devices = false
          base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = ${systemd_cgroup}
  {{- if .nodeGroup.gpu }}
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
          privileged_without_host_devices = false
          runtime_engine = ""
          runtime_root = ""
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
            BinaryName = "/usr/bin/nvidia-container-runtime"
            SystemdCgroup = ${systemd_cgroup}
  {{- end }}
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
      conf_dir = "/etc/cni/net.d"
      max_conf_num = 1
      conf_template = ""
    [plugins."io.containerd.grpc.v1.cri".registry]
{{- if .registry.registryModuleEnable }}
    config_path = "/etc/containerd/registry.d"
{{- else }}
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]
  {{- range $host_name, $host_values := .registry.hosts }}
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ $host_name }}"]
          endpoint = [{{- range $i, $mirror := $host_values.mirrors }}{{ if $i }}, {{ end }}{{ printf "%s://%s" $mirror.scheme $mirror.host| quote }}{{- end }}]
  {{- end }}
  {{- range $host_name, $host_values := .registry.hosts }}
    {{- range $mirror := $host_values.mirrors }}
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ $mirror.host }}".auth]
          {{- if (($mirror).auth).username }}
          username = {{ $mirror.auth.username | quote }}
          password = {{ $mirror.auth.password | default "" | quote }}
          {{- else }}
          auth = {{ (($mirror).auth).auth | default "" | quote }}
          {{- end }}
      {{- if $mirror.ca }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ $mirror.host }}".tls]
          ca_file = "/opt/deckhouse/share/ca-certificates/registry-{{ $mirror.host | lower }}-ca.crt"
      {{- end }}
      {{- if eq $mirror.scheme "http" }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ $mirror.host }}".tls]
          insecure_skip_verify = true
      {{- end }}
    {{- end }}
  {{- end }}
  {{- if eq .runType "Normal" }}
    {{- range $registryAddr,$ca := .normal.moduleSourcesCA }}
      {{- if $ca }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ $registryAddr | lower }}".tls]
          ca_file = "/opt/deckhouse/share/ca-certificates/{{ $registryAddr | lower }}-ca.crt"
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
    [plugins."io.containerd.grpc.v1.cri".image_decryption]
      key_model = ""
    [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
      tls_cert_file = ""
      tls_key_file = ""
  [plugins."io.containerd.internal.v1.opt"]
    path = "/opt/containerd"
  [plugins."io.containerd.internal.v1.restart"]
    interval = "10s"
  [plugins."io.containerd.metadata.v1.bolt"]
    content_sharing_policy = "shared"
  [plugins."io.containerd.monitor.v1.cgroups"]
    no_prometheus = false
  [plugins."io.containerd.runtime.v1.linux"]
    shim = "containerd-shim"
    runtime = "runc"
    runtime_root = ""
    no_shim = false
    shim_debug = false
  [plugins."io.containerd.runtime.v2.task"]
    platforms = ["linux/amd64"]
  [plugins."io.containerd.service.v1.diff-service"]
    default = ["walking"]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    root_path = ""
    pool_name = ""
    base_image_size = ""
    async_remove = false
EOF
{{- end }}


additional_configs() {
  local conf_dir="$1"
  local unusable_conf_dir="$2"
  local root_path="/etc/containerd"
  local full_conf_path="$root_path/$conf_dir"

  rm -rf "$root_path/$unusable_conf_dir/"
  
  if ls "${full_conf_path}/"*.toml >/dev/null 2>/dev/null; then
    toml-merge "$root_path/deckhouse.toml" "${full_conf_path}/"*.toml -
  else
    cat "$root_path/deckhouse.toml"
  fi
}

check_additional_configs() {
  local full_conf_path="$1"
  local ctrd_version="$2"

  if ls ${full_conf_path}/*.toml >/dev/null 2>&1; then
    for path in ${full_conf_path}/*.toml; do
      if [ "$ctrd_version" = "v1" ]; then
        if bb-ctrd-v1-has-registry-fields "${path}"; then
          >&2 echo "Failed to merge $path: contains custom registry fields; please configure them in /etc/containerd/registry.d"
          exit 1
        fi
      fi
      if [ "$ctrd_version" = "v2" ]; then
        if bb-ctrd-v2-has-registry-fields "${path}"; then
          >&2 echo "Failed to merge $path: contains custom registry fields; please configure them in /etc/containerd/registry.d"
          exit 1
        fi
      fi
    done
  fi
}

# Check additional configs
{{- if eq .cri "ContainerdV2" }}
check_additional_configs /etc/containerd/conf2.d "v2"
containerd_toml=$(additional_configs conf2.d conf.d)
{{- else if eq .cri "Containerd" }}
  {{- if .registry.registryModuleEnable }}
check_additional_configs /etc/containerd/conf.d "v1"
  {{- end }}
containerd_toml=$(additional_configs conf.d conf2.d)
{{- end }}

bb-sync-file /etc/containerd/config.toml - containerd-config-file-changed <<< "${containerd_toml}"

bb-sync-file /etc/crictl.yaml - << "EOF"
runtime-endpoint: unix:/var/run/containerd/containerd.sock
image-endpoint: unix:/var/run/containerd/containerd.sock
timeout: 2
debug: false
pull-image-on-create: false
EOF
{{- end }}

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
if bb-flag? cntrd-major-version-changed; then
  migrate
fi
{{- end }}