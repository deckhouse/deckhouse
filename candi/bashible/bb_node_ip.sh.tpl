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
    echo "Cannot discover internal network CIDRs. Node has more than one interface, and StaticClusterConfiguration internalNetworkCIDRs is not set." >&2
    return 1
  fi
}

function check_slash32_node_ip() {
  local inet_lines
  inet_lines="$(ip -4 -o addr show up scope global | awk '$2 != "lo" {print $4}')"
  if [[ -z "$inet_lines" || "$(wc -l <<< "${inet_lines}")" -ne 1 ]]; then
    return 1
  fi

  local inet_line="${inet_lines}"
  local addr="${inet_line%/*}"
  local prefix="${inet_line#*/}"

  [[ "$prefix" == "32" ]] && echo "$addr"
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
    bb-log-error "Unable to discover node IP for node object: No access to API server"
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

  python3 - << 'EOF' > /var/lib/bashible/discovered-node-ip.tmp
  import sys
  import ipaddress
  import subprocess
  import os

  def get_system_ips_and_prefixes():
      ips = []
      try:
          output = subprocess.check_output(['ip', '-o', 'addr', 'show', 'up', 'scope', 'global'], universal_newlines=True)
          for line in output.splitlines():
              parts = line.split()
              if len(parts) >= 4 and parts[1] != 'lo':
                  addr_with_prefix = parts[3]
                  if '/' in addr_with_prefix:
                      ip, prefix = addr_with_prefix.split('/')
                      try:
                          ipaddress.ip_address(ip)
                          ips.append((ip, prefix))
                      except ValueError:
                          pass
      except Exception as e:
          pass
      return ips

  def is_ip_in_cidr(ip_str, cidr_str):
      try:
          ip = ipaddress.ip_address(ip_str)
          net = ipaddress.ip_network(cidr_str, strict=False)
          return ip in net
      except ValueError:
          return False

  def discover_ip(internal_network_cidrs_str):
      system_ips_with_prefixes = get_system_ips_and_prefixes()
    
      if not internal_network_cidrs_str:
          matched_ipv4 = None
          matched_ipv6 = None
          for ip, prefix in system_ips_with_prefixes:
              try:
                  obj = ipaddress.ip_address(ip)
                  if obj.version == 4 and prefix == '32' and not matched_ipv4:
                      matched_ipv4 = ip
                  elif obj.version == 6 and prefix == '128' and not matched_ipv6:
                      matched_ipv6 = ip
              except:
                  pass
        
          final_ips = [ip for ip in [matched_ipv4, matched_ipv6] if ip]
          if final_ips:
              return ",".join(final_ips)
          return ""
        
      cidrs = internal_network_cidrs_str.split()
      matched_ipv4 = None
      matched_ipv6 = None
    
      for cidr in cidrs:
          for ip, prefix in system_ips_with_prefixes:
              if is_ip_in_cidr(ip, cidr):
                  try:
                      obj = ipaddress.ip_address(ip)
                      if obj.version == 4 and not matched_ipv4:
                          matched_ipv4 = ip
                      elif obj.version == 6 and not matched_ipv6:
                          matched_ipv6 = ip
                  except:
                      pass
    
      final_ips = [ip for ip in [matched_ipv4, matched_ipv6] if ip]
      if final_ips:
          return ",".join(final_ips)
      return ""

  if __name__ == '__main__':
      cidrs = os.environ.get('internal_network_cidrs', "")
      res = discover_ip(cidrs)
      if res:
          print(res)
  EOF

  if [ -s /var/lib/bashible/discovered-node-ip.tmp ]; then
    mv /var/lib/bashible/discovered-node-ip.tmp /var/lib/bashible/discovered-node-ip
    exit 0
  else
    rm -f /var/lib/bashible/discovered-node-ip.tmp
  fi
  {{- end }}

