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

{{- if eq .cri "Containerd" }}
_on_containerd_config_changed() {
  bb-flag-set containerd-need-restart
}

bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

  {{- $max_concurrent_downloads := 3 }}
  {{- if hasKey .nodeGroup.cri "containerd" }}
    {{- $max_concurrent_downloads = .nodeGroup.cri.containerd.maxConcurrentDownloads | default $max_concurrent_downloads }}
  {{- end }}
  {{- $sandbox_image := "registry.k8s.io/pause:3.2" }}
  {{- if .images }}
    {{- if .images.common.pause }}
      {{- $sandbox_image = printf "%s%s@%s" .registry.address .registry.path .images.common.pause }}
    {{- end }}
  {{- end }}

bb-sync-file /etc/containerd/certs.d/_default/hosts.toml - << EOF
[host."https://registry-1.docker.io"]
  capabilities = ["pull", "resolve"]

[host."{{ .registry.scheme }}://{{ .registry.address }}"]
  capabilities = ["pull", "resolve"]
  {{- if .registry.ca }}
  ca = ["/opt/deckhouse/share/ca-certificates/registry-ca.crt"]
  {{- end }}

  {{- if eq .registry.scheme "http" }}
  skip_verify = true
  {{- end }}

  {{- if eq .runType "Normal" }}
    {{- range $registryAddr,$ca := .normal.moduleSourcesCA }}
      {{- if $ca }}
[host."https://{{ $registryAddr | lower }}"]
  ca = "/opt/deckhouse/share/ca-certificates/{{ $registryAddr | lower }}-ca.crt"
      {{- end }}
    {{- end }}
  {{- end }}
EOF

  {{- $with_auth := "" }}
  {{- with .registry.auth }}
    {{- $with_auth = printf "--auth %s" . -}}
  {{- end }}

crictl pull {{ $with_auth }} {{ $sandbox_image }}

systemd_cgroup=true
# Overriding cgroup type from external config file
if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
  systemd_cgroup=false
fi

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
      config_path = '/etc/containerd/certs.d'

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
      default_runtime_name = "runc"
      ignore_blockio_not_enabled_errors = false
      ignore_rdt_not_enabled_errors = false

      [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes]
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

  [plugins."io.containerd.snapshotter.v1.erofs"]
      # Enable fsverity support for EROFS layers, default is false
      # Temporary disable fsverity
      enable_fsverity = false

      # Optional: Additional mount options for overlayfs
      ovl_mount_options = []

  [plugins."io.containerd.service.v1.diff-service"]
    default = ["erofs","walking"]

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
    enable_tls_streaming = false

    [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
      tls_cert_file = ""
      tls_key_file = ""

  [plugins."io.containerd.internal.v1.opt"]
    path = "/opt/containerd"

  [plugins."io.containerd.metadata.v1.bolt"]
    content_sharing_policy = "shared"

  [plugins.'io.containerd.monitor.container.v1.restart']
    interval = "10s"
    
  [plugins.'io.containerd.monitor.task.v1.cgroups']
    no_prometheus = false

  [plugins."io.containerd.runtime.v2.task"]
    platforms = ["linux/amd64"]

  [plugins.'io.containerd.service.v1.tasks-service']
    blockio_config_file = ''
    rdt_config_file = ''

  [plugins.'io.containerd.shim.v1.manager']
    env = []
    
[cgroup]
  path = ""

[timeouts]
  "io.containerd.timeout.shim.cleanup" = "5s"
  "io.containerd.timeout.shim.load" = "5s"
  "io.containerd.timeout.shim.shutdown" = "3s"
  "io.containerd.timeout.task.state" = "2s"
EOF

# Check additional configs
if ls /etc/containerd/conf.d/*.toml >/dev/null 2>/dev/null; then
  containerd_toml="$(toml-merge /etc/containerd/deckhouse.toml /etc/containerd/conf.d/*.toml -)"
else
  containerd_toml="$(cat /etc/containerd/deckhouse.toml)"
fi

bb-sync-file /etc/containerd/config.toml - containerd-config-file-changed <<< "${containerd_toml}"

bb-sync-file /etc/crictl.yaml - << "EOF"
runtime-endpoint: unix:/var/run/containerd/containerd.sock
image-endpoint: unix:/var/run/containerd/containerd.sock
timeout: 2
debug: false
pull-image-on-create: false
EOF
{{- end }}
