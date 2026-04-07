# Copyright 2023 Flant JSC
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
{{- if or (contains "Static" .nodeGroup.nodeType) (eq .runType "ClusterBootstrap") }}
bb-sync-file /var/lib/bashible/cleanup_static_node.sh - << "EOF"
#!/bin/bash

MOTD_FILE="/etc/motd"
MARKER="D8_CLEANUP_STATIC_NODE"
CLEANUP_FAILED=0
SCRIPT_PATH="/var/lib/bashible/cleanup_static_node.sh"
SCRIPT_BACKUP="/tmp/cleanup_static_node.sh.bak"

PATHS_TO_REMOVE=(
  /var/cache/registrypackages
  /etc/kubernetes
  /var/lib/kubelet
  /var/lib/containerd
  /etc/cni
  /var/lib/cni
  /opt/cni
  /var/lib/etcd
  /opt/containerd
  /etc/containerd
  /opt/deckhouse
  /var/lib/deckhouse
  /var/log/kube-audit
  /var/log/pods
  /var/log/containers
  /var/log/containerd
  /etc/logrotate.d/containerd-integrity.conf
  /var/lib/upmeter
  /etc/sudoers.d/sudoers_flant_kubectl
  /etc/sudoers.d/30-deckhouse-nodeadmins
  /home/deckhouse
)

SERVICES_TO_REMOVE=(
  bashible.service
  bashible.timer
  d8-shutdown-inhibitor.service
  sysctl-tuner.service
  sysctl-tuner.timer
  old-csi-mount-cleaner.service
  old-csi-mount-cleaner.timer
  d8-containerd-cgroup-migration.service
  containerd-deckhouse.service
  containerd-deckhouse-logger.service
  containerd-deckhouse-logger-logrotate.service
  containerd-deckhouse-logger-logrotate.timer
  kubelet.service
)

SYSTEMD_FILES=(
  /etc/systemd/system/bashible.*
  /etc/systemd/system/sysctl-tuner.*
  /etc/systemd/system/old-csi-mount-cleaner.*
  /etc/systemd/system/d8-containerd-cgroup-migration.*
  /etc/systemd/system/containerd-deckhouse*
  /lib/systemd/system/containerd-deckhouse*
  /etc/systemd/system/d8-shutdown-inhibitor*
  /lib/systemd/system/d8-shutdown-inhibitor*
  /etc/systemd/logind.conf.d/99-node-d8-shutdown-inhibitor.conf
  /etc/systemd/system/kubelet*
  /lib/systemd/system/kubelet*
)

log_info() {
  echo "[INFO] $(date +'%Y-%m-%d %H:%M:%S') - $@"
}

log_err() {
  echo "[ERROR] $(date +'%Y-%m-%d %H:%M:%S') - $@" >&2
}

restore_motd_message() {
  sed -i "\|^# ${MARKER}_START$|,\|^# ${MARKER}_END$|d" "$MOTD_FILE" 2>/dev/null || true
}

set_motd_message() {
  restore_motd_message
  cat <<BLOCK >> "$MOTD_FILE"
# ${MARKER}_START
Deckhouse node cleanup is not complete. Reboot and run:
  bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
If you see this message by mistake, please remove it from /etc/motd.
# ${MARKER}_END
BLOCK
}

stop_services() {
  log_info "systemctl stop $@"
  systemctl stop $@ 2>/dev/null || true
}

kill_and_wait() {
  local pattern=$1

  log_info "Stopping processes matching pattern: $pattern"
  # Try SIGTERM
  pkill -f "$pattern" || true
  for i in {1..5}; do
    pgrep -f "$pattern" >/dev/null || return 0
    sleep 1
  done

  # Try SIGKILL
  pkill -9 -f "$pattern" || true
  for i in {1..5}; do
    pgrep -f "$pattern" >/dev/null || return 0
    sleep 1
  done

  # Check
  log_err "ERROR: Process '$pattern' still running after SIGKILL."
  CLEANUP_FAILED=1
  return 1
}

remove_path() {
  local path="$1"

  # if it does not exist
  [ ! -e "$path" ] && return 0

  for i in {1..5}; do
    if [ -d "$path" ]; then
      mount | grep -F "$path" | awk '{print $3}' | sort -r | xargs -r umount -l 2>/dev/null
    fi
    rm -rf "$path" 2>/dev/null && return 0
    sleep 1
  done

  # if it exists after attempting to delete
  if [ -e "$path" ]; then
    log_err "ERROR: failed to remove $path"
    return 1
  fi
}

# --- Main ---
log_info "Starting static node cleanup"

# Backup current script for potential rerun
cp -f "$0" "$SCRIPT_BACKUP" 2>/dev/null || log_err "Failed to backup cleanup script to $SCRIPT_BACKUP"
chmod +x "$SCRIPT_BACKUP" 2>/dev/null || true

if [ "$1" != "--yes-i-am-sane-and-i-understand-what-i-am-doing" ]; then
  log_err "Needed flag isn't passed, exit without any action (--yes-i-am-sane-and-i-understand-what-i-am-doing)"
  exit 1
fi

log_info "Setting MOTD cleanup message"
set_motd_message

# Stop services
log_info "Stopping services"
for service in "${SERVICES_TO_REMOVE[@]}"; do
  stop_services "$service"
done

# Kill Processes
kill_and_wait "bash /var/lib/bashible/bashible"
kill_and_wait "containerd-shim"

# Unmount
log_info "Unmounting kubelet/containerd mounts"
for dir in /var/lib/kubelet /var/lib/containerd; do
  mount | grep "$dir" | awk '{print $3}' | sort -r | xargs -r umount -l 2>/dev/null
done

# Remove immutable bit
if [ -d /var/lib/containerd/io.containerd.snapshotter.v1.erofs ]; then
  chattr -R -i /var/lib/containerd/io.containerd.snapshotter.v1.erofs 2>/dev/null || true
fi

# Remove systemd files
log_info "Removing systemd unit files and reloading systemd"
rm -rf "${SYSTEMD_FILES[@]}"
systemctl daemon-reload
systemctl -s SIGHUP kill systemd-logind

# Remove files
for p in "${PATHS_TO_REMOVE[@]}"; do
  log_info "Removing $p"
  remove_path "$p" || CLEANUP_FAILED=1
done

# Remove Users
log_info "Removing users"
userdel deckhouse 2>/dev/null
groupdel nodeadmin 2>/dev/null
grep "created by deckhouse" /etc/passwd | cut -d: -f1 | xargs -r -n1 userdel 2>/dev/null

# Handle d8-dhctl-converger cleanup
if getent passwd d8-dhctl-converger >/dev/null; then
  log_info "Scheduling d8-dhctl-converger cleanup on reboot"
  cat <<'EOF_CRON' > /root/d8-user-cleanup.sh
#!/bin/bash
userdel d8-dhctl-converger
[ -f /root/old_crontab ] && crontab /root/old_crontab && rm -f /root/old_crontab
rm -f "$0"
EOF_CRON
  chmod +x /root/d8-user-cleanup.sh
  crontab -l 2>/dev/null > /root/old_crontab
  (cat /root/old_crontab; echo "@reboot /root/d8-user-cleanup.sh") | crontab -
fi

remove_path /var/lib/bashible/ || CLEANUP_FAILED=1

if [ "$CLEANUP_FAILED" -ne 0 ]; then
  if [ ! -f "$SCRIPT_PATH" ]; then
    mkdir -p /var/lib/bashible/
    cp -f "$SCRIPT_BACKUP" "$SCRIPT_PATH"
    chmod +x "$SCRIPT_PATH"
  fi
  log_err "Cleanup finished with errors. Reboot the server and run as root user $SCRIPT_PATH --yes-i-am-sane-and-i-understand-what-i-am-doing again, or fix the issues above manually"
  exit 2
fi

log_info "Cleanup completed successfully, restoring MOTD and rebooting"
restore_motd_message
shutdown -r -t 5
EOF
{{- end }}
