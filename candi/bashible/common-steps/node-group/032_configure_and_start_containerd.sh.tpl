# Copyright 2021 Flant JSC
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
  {{ if ne .runType "ImageBuilding" -}}
  bb-flag-set containerd-need-restart
  bb-flag-set kubelet-need-restart
  {{- end }}
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

systemd_cgroup=true
# Overriding cgroup type from external config file
if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
  systemd_cgroup=false
fi

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
      default_runtime_name = "runc"
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
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
      conf_dir = "/etc/cni/net.d"
      max_conf_num = 1
      conf_template = ""
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io"]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ .registry.address }}"]
          endpoint = ["{{ .registry.scheme }}://{{ .registry.address }}"]
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".auth]
          auth = "{{ .registry.auth | default "" }}"
  {{- if eq .registry.scheme "http" }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".tls]
          insecure_skip_verify = true
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

if bb-flag? containerd-need-restart; then
  bb-log-warning "'containerd-need-restart' flag was set. Containerd should be restarted!"
  {{ if ne .runType "ImageBuilding" -}}
  systemctl restart containerd-deckhouse.service
  {{- end }}
  bb-flag-unset containerd-need-restart
fi

{{- end }}
