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

###############################################
#  CASE 1: swapBehavior is empty or == NoSwap
###############################################
{{- if or (eq $swapBehavior "") (eq $swapBehavior "NoSwap") }}

# Stop and mask systemd swap units
for swapunit in $(systemctl list-units --no-legend --plain --no-pager --type swap | cut -f1 -d" "); do
  systemctl stop "$swapunit"
  systemctl mask "$swapunit"
done

# systemd-gpt-auto-generator automatically detects swap partition in GPT and activates it
if [ -f /lib/systemd/system-generators/systemd-gpt-auto-generator ] && ( [ ! -L /etc/systemd/system-generators/systemd-gpt-auto-generator ] || [ "$(readlink -f /etc/systemd/system-generators/systemd-gpt-auto-generator)" != "/dev/null" ] ); then
  mkdir -p /etc/systemd/system-generators
  ln -sf /dev/null /etc/systemd/system-generators/systemd-gpt-auto-generator
fi

swapoff -a || true

# Remove swapfile if present
if [ -f "$SWAPFILE" ]; then
  rm -f "$SWAPFILE"
fi

if grep -q "swap" /etc/fstab; then
  sed -i '/[[:space:]]swap[[:space:]]/d' /etc/fstab
fi

# Set swappiness to 0 when swap is disabled
sysctl -w vm.swappiness=0
mkdir -p /etc/sysctl.d
echo "vm.swappiness=0" > "$SYSCTL_CONF"

exit 0
{{- end }}

###############################################
#  CASE 2: swapBehavior == LimitedSwap
###############################################
{{- if eq $swapBehavior "LimitedSwap" }}

if [ -z "{{ $limitedSwapSize }}" ]; then
  echo "ERROR: limitedSwap.size is required when swapBehavior=LimitedSwap"
  exit 1
fi

SIZE="{{ $limitedSwapSize }}"

# Convert human size to bytes for comparison
bytes() {
  local s=$1
  local n=${s%[KkMmGgIi]*}
  local u=${s#${n}}

  case "$u" in
    Ki | ki | K | k) echo $((n * 1024));;
    Mi | mi | M | m) echo $((n * 1024 * 1024));;
    Gi | gi | G | g) echo $((n * 1024 * 1024 * 1024));;
    *) echo "$n";;
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
  swapoff -a || true
  rm -f "$SWAPFILE"
  # Use bytes for fallocate (more reliable), fallback to dd if fallocate fails
  fallocate -l "$DESIRED_BYTES" "$SWAPFILE" || dd if=/dev/zero of="$SWAPFILE" bs=1M count=$((DESIRED_BYTES / 1024 / 1024))
  chmod 600 "$SWAPFILE"
  mkswap "$SWAPFILE"
fi

# Ensure fstab entry exists
if ! grep -q "$SWAPFILE" /etc/fstab; then
  echo "$SWAPFILE none swap sw 0 0" >> /etc/fstab
fi

# Enable swap
swapon "$SWAPFILE" || true

# Configure swappiness
SWAPPINESS="{{ $swappiness }}"
sysctl -w vm.swappiness="$SWAPPINESS"
mkdir -p /etc/sysctl.d
echo "vm.swappiness=$SWAPPINESS" > "$SYSCTL_CONF"

exit 0
{{- end }}

###############################################
#  CASE 3: swapBehavior == UnlimitedSwap
###############################################
{{- if eq $swapBehavior "UnlimitedSwap" }}

# For unlimited swap, just ensure swap is enabled
# but don't manage swapfile ourselves
swapon -a || true

# Configure swappiness
SWAPPINESS="{{ $swappiness }}"
sysctl -w vm.swappiness="$SWAPPINESS"
mkdir -p /etc/sysctl.d
echo "vm.swappiness=$SWAPPINESS" > "$SYSCTL_CONF"

exit 0
{{- end }}

