{{- /*
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
*/}}

function discover_internal_network_cidrs() {
  local physical_iface
  local discovered_internal_network_cidrs

  physical_iface="$(ls -l /sys/class/net/ | grep -vE "virtual|total" | grep "devices" | awk '{print $9}')"
  if [[ "$(wc -l <<< "${physical_iface}")" -eq 1 ]]; then
    discovered_internal_network_cidrs="$(ip route show scope link proto kernel dev "${physical_iface}" | awk '{print $1}')"
    echo "$discovered_internal_network_cidrs"
  else
    echo "Cannot discover internal network CIDRs: node has more than one interface and StaticClusterConfiguration.internalNetworkCIDRs is not set" >&2
    return 1
  fi
}

# Ensure we have file
touch /var/lib/bashible/discovered-node-ip

{{- if ne .nodeGroup.nodeType "Static" }}
  {{ if eq .runType "ClusterBootstrap" }}
    {{- if and .clusterBootstrap.cloud .clusterBootstrap.cloud.nodeIP }}
echo {{ .clusterBootstrap.cloud.nodeIP }} > /var/lib/bashible/discovered-node-ip
    {{- end }}
  # For CloudEphemeral, CloudPermanent or CloudStatic node we try to discover IP from Node object
  {{- else }}
if [ -f /etc/kubernetes/kubelet.conf ] ; then
  if node="$(bb-curl-kube "/api/v1/nodes/$(bb-d8-node-name)" 2> /dev/null)" ; then
    echo "$node" | jq -r '([.status.addresses[] | select(.type == "InternalIP") | .address] + [.status.addresses[] | select(.type == "ExternalIP") | .address]) as $ips | (($ips | map(select(test(":") | not)) | .[0]) // null) as $v4 | (($ips | map(select(test(":"))) | .[0]) // null) as $v6 | [$v4, $v6] | map(select(. != null)) | join(",")' > /var/lib/bashible/discovered-node-ip
  else
    bb-log-error "Cannot discover node IP from Node object: Kubernetes API server is unreachable"
    exit 1
  fi
fi
  {{- end }}
{{- end }}

{{- if eq .nodeGroup.nodeType "Static" }}
  {{- if and (hasKey .nodeGroup "static") (hasKey .nodeGroup.static "internalNetworkCIDRs")}}
internal_network_cidrs={{ .nodeGroup.static.internalNetworkCIDRs | join " " | quote }}
  {{- end }}

if [[ -z "$internal_network_cidrs" ]]; then
  internal_network_cidrs="$(discover_internal_network_cidrs || true)"
fi

# Pass CIDR list to Python via environment.
# Python script discovers node IPs (IPv4 and/or IPv6) that match the given CIDRs.
# If no CIDRs are provided, it falls back to the DVP-like case: single /32 (IPv4) or /128 (IPv6).
export internal_network_cidrs
internal_network_cidrs="${internal_network_cidrs}" python3 - > /var/lib/bashible/discovered-node-ip.tmp << 'PYEOF'
import ipaddress
import os
import subprocess


def get_system_ips_and_prefixes():
    ips = []
    try:
        output = subprocess.check_output(
            ['ip', '-o', 'addr', 'show', 'up', 'scope', 'global'],
            universal_newlines=True,
        )
    except Exception:
        return ips

    for line in output.splitlines():
        parts = line.split()
        if len(parts) < 4 or parts[1] == 'lo':
            continue
        addr_with_prefix = parts[3]
        if '/' not in addr_with_prefix:
            continue
        ip, prefix = addr_with_prefix.split('/', 1)
        try:
            ipaddress.ip_address(ip)
        except ValueError:
            continue
        ips.append((ip, prefix))
    return ips


def is_ip_in_cidr(ip_str, cidr_str):
    try:
        ip = ipaddress.ip_address(ip_str)
        net = ipaddress.ip_network(cidr_str, strict=False)
    except ValueError:
        return False
    return ip in net


def discover_ip(internal_network_cidrs_str):
    system_ips = get_system_ips_and_prefixes()
    matched_v4 = None
    matched_v6 = None

    if not internal_network_cidrs_str:
        # DVP-like fallback: a single /32 IPv4 or /128 IPv6 address.
        for ip, prefix in system_ips:
            try:
                version = ipaddress.ip_address(ip).version
            except ValueError:
                continue
            if version == 4 and prefix == '32' and matched_v4 is None:
                matched_v4 = ip
            elif version == 6 and prefix == '128' and matched_v6 is None:
                matched_v6 = ip
    else:
        for cidr in internal_network_cidrs_str.split():
            for ip, _ in system_ips:
                if not is_ip_in_cidr(ip, cidr):
                    continue
                try:
                    version = ipaddress.ip_address(ip).version
                except ValueError:
                    continue
                if version == 4 and matched_v4 is None:
                    matched_v4 = ip
                elif version == 6 and matched_v6 is None:
                    matched_v6 = ip

    final_ips = [ip for ip in (matched_v4, matched_v6) if ip]
    return ",".join(final_ips)


if __name__ == '__main__':
    cidrs = os.environ.get('internal_network_cidrs', '')
    res = discover_ip(cidrs)
    if res:
        print(res)
PYEOF

if [ -s /var/lib/bashible/discovered-node-ip.tmp ]; then
  mv /var/lib/bashible/discovered-node-ip.tmp /var/lib/bashible/discovered-node-ip
  exit 0
else
  rm -f /var/lib/bashible/discovered-node-ip.tmp
  bb-log-error "Unable to discover node IP: no system address matches internal_network_cidrs='${internal_network_cidrs}'"
  exit 1
fi
{{- end }}
