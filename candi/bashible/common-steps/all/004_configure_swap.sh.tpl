# Copyright 2025 Flant JSC
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

{{- $swapBehavior := dig "kubelet" "memorySwap" "swapBehavior" "" .nodeGroup }}
{{- $limitedSwapSize := dig "kubelet" "memorySwap" "limitedSwap" "size" "" .nodeGroup }}
{{- $swappiness := dig "kubelet" "memorySwap" "swappiness" 60 .nodeGroup }}

SWAPFILE="/var/lib/swapfile"
SYSCTL_CONF="/etc/sysctl.d/99-swap.conf"


{{- if or (eq $swapBehavior "") (eq $swapBehavior "NoSwap") }}
###############################################
#  CASE 1: swapBehavior is empty or == NoSwap
###############################################

# Stop and mask systemd swap units
for swapunit in $(systemctl list-units --no-legend --plain --no-pager --type swap | cut -f1 -d" "); do
  systemctl stop "$swapunit" || true
  systemctl mask "$swapunit" || true
done

# systemd-gpt-auto-generator automatically detects swap partition in GPT and activates it
if [ -f /lib/systemd/system-generators/systemd-gpt-auto-generator ] && ( [ ! -L /etc/systemd/system-generators/systemd-gpt-auto-generator ] || [ "$(readlink -f /etc/systemd/system-generators/systemd-gpt-auto-generator)" != "/dev/null" ] ); then
  mkdir -p /etc/systemd/system-generators
  ln -sf /dev/null /etc/systemd/system-generators/systemd-gpt-auto-generator
fi

# Disable any active swap, no need to restart kubelet
swapoff -a || true

# Remove swapfile if present
if [ -f "$SWAPFILE" ]; then
  rm -f "$SWAPFILE"
fi

if grep -q "[[:space:]]swap[[:space:]]" /etc/fstab; then
  sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab
fi

# Set swappiness to 0 when swap is disabled
sysctl -w vm.swappiness=0
mkdir -p /etc/sysctl.d
echo "vm.swappiness=0" > "$SYSCTL_CONF"

exit 0
{{- end }}


{{- if eq $swapBehavior "LimitedSwap" }}
###############################################
#  CASE 2: swapBehavior == LimitedSwap
###############################################

SKIP_SWAP_CONFIGURATION=false

# Helper functions
version_ge() { [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }
has_cgroup2() { [ "$(stat -f -c %T /sys/fs/cgroup 2>/dev/null)" = "cgroup2fs" ]; }

# Check Kubernetes version (minimum 1.22 for swap support)
KUBE_VERSION="{{ .kubernetesVersion }}"
if ! version_ge "$KUBE_VERSION" "1.22"; then
  bb-log-warning "Swap support requires Kubernetes >= 1.22, current version is $KUBE_VERSION. Skipping swap configuration."
  SKIP_SWAP_CONFIGURATION=true
fi

# Check cgroup v2 (swap only works with cgroupv2)
if ! has_cgroup2; then
  bb-log-warning "Swap support requires cgroup v2, but this node uses cgroup v1. Skipping swap configuration."
  SKIP_SWAP_CONFIGURATION=true
fi

# Exit if preconditions are not met
if [ "$SKIP_SWAP_CONFIGURATION" = "true" ]; then
  exit 0
fi

if [ -z "{{ $limitedSwapSize }}" ]; then
  bb-log-error "Error getting limitedSwap.size"
  exit 1
fi

SIZE="{{ $limitedSwapSize }}"
bb-log-info "Configuring LimitedSwap with size: $SIZE"

# Convert human size to bytes for comparison
bytes() {
  local s=$1
  local n u
  
  # Extract number and unit
  n=$(echo "$s" | sed 's/[^0-9].*$//')
  u=$(echo "$s" | sed 's/^[0-9]*//')

  # Validate: number must exist
  if [ -z "$n" ]; then
    bb-log-error "Invalid size format (no number): $s"
    return 1
  fi

  case "$u" in
    Gi|gi|G|g) echo $((n * 1024 * 1024 * 1024));;
    Mi|mi|M|m) echo $((n * 1024 * 1024));;
    Ki|ki|K|k) echo $((n * 1024));;
    "") echo "$n";;
    *) bb-log-error "Unknown size unit '$u' in: $s"; return 1;;
  esac
}

DESIRED_BYTES=$(bytes "$SIZE")
CURRENT_BYTES=0

if [ -f "$SWAPFILE" ]; then
  # Try Linux stat first, fall back to generic approach
  if stat -c%s "$SWAPFILE" >/dev/null 2>&1; then
    CURRENT_BYTES=$(stat -c%s "$SWAPFILE")
  else
    CURRENT_BYTES=$(stat -f%z "$SWAPFILE" 2>/dev/null || ls -l "$SWAPFILE" | awk '{print $5}')
  fi
fi

# Recreate swapfile if size differs or doesn't exist
if [ "$CURRENT_BYTES" -ne "$DESIRED_BYTES" ]; then
  bb-log-info "Creating swapfile: current=${CURRENT_BYTES} bytes, desired=${DESIRED_BYTES} bytes"
  swapoff "$SWAPFILE" 2>/dev/null || true
  rm -f "$SWAPFILE"
  # Use bytes for fallocate (more reliable), fallback to dd if fallocate fails
  if fallocate -l "$DESIRED_BYTES" "$SWAPFILE"; then
    bb-log-info "Swapfile created with fallocate"
  else
    bb-log-info "fallocate failed, using dd as fallback"
    dd if=/dev/zero of="$SWAPFILE" bs=1M count=$((DESIRED_BYTES / 1024 / 1024))
  fi
  chmod 600 "$SWAPFILE"
  mkswap "$SWAPFILE"
  bb-log-info "Swapfile formatted successfully"
  # Swapfile changed, kubelet needs restart
  bb-flag-set kubelet-need-restart
else
  bb-log-info "Swapfile already exists with correct size: ${CURRENT_BYTES} bytes"
fi

# Ensure fstab entry exists
if ! grep -q "$SWAPFILE" /etc/fstab; then
  echo "$SWAPFILE none swap sw 0 0" >> /etc/fstab
fi

# Enable swap if not already active
if swapon --show | grep -q "$SWAPFILE"; then
  bb-log-info "Swap already active"
else
  if swapon "$SWAPFILE"; then
    bb-log-info "Swap enabled successfully"
    # Swap was just enabled, kubelet needs restart
    bb-flag-set kubelet-need-restart
  else
    bb-log-error "Failed to enable swap"
    exit 1
  fi
fi

# Configure swappiness
SWAPPINESS="{{ $swappiness }}"
sysctl -w vm.swappiness="$SWAPPINESS"
mkdir -p /etc/sysctl.d
echo "vm.swappiness=$SWAPPINESS" > "$SYSCTL_CONF"

exit 0
{{- end }}
