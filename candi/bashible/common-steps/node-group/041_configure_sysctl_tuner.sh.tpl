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

bb-event-on 'sysctl-tuner-service-changed' '_enable_sysctl_tuner_service'
function _enable_sysctl_tuner_service() {
  systemctl daemon-reload
  systemctl enable sysctl-tuner.timer
}

bb-sync-file /opt/deckhouse/bin/sysctl-tuner - << "EOF"
#!/bin/bash

# After multiplying this value by the number of cores of the node, we get `nf_conntrack_max`,
# but do not apply it until we process the parameter below
CONNTRACK_MAX_PER_CORE=131072
# If this value turns out to be greater than the `nf_conntrack_max` obtained above, then it is applied
CONNTRACK_MIN=524288

CPU_NUM=`cat /proc/cpuinfo | grep -E '^processor\s+:\s+[0-9]+$' | wc -l`
CONNTRACK_BY_CPU=$(( $CPU_NUM * $CONNTRACK_MAX_PER_CORE ))
NF_CONNTRACK_MAX=$(( $CONNTRACK_BY_CPU > $CONNTRACK_MIN ? $CONNTRACK_BY_CPU : $CONNTRACK_MIN ))

sysctl -w net.netfilter.nf_conntrack_max=$NF_CONNTRACK_MAX # set a limit on the number of conntracks
sysctl -w net.nf_conntrack_max=$NF_CONNTRACK_MAX
echo $(( $NF_CONNTRACK_MAX / 4 )) > /sys/module/nf_conntrack/parameters/hashsize # set the proportional size of the hash table for search by contact

# Prevent ipv4 forwarding from being disabled
sysctl -w net.ipv4.conf.all.forwarding=1

# http://www.brendangregg.com/blog/2017-12-31/reinvent-netflix-ec2-tuning.html
sysctl -w vm.swappiness=0
sysctl -w net.core.somaxconn=1000
sysctl -w net.core.netdev_max_backlog=5000 # increase the backlog of packets taken from the ring buffer of the network card, but not yet transmitted up the network stack kernel
sysctl -w net.core.rmem_max=16777216
sysctl -w net.core.wmem_max=16777216
sysctl -w net.ipv4.tcp_wmem="4096 12582912 16777216"
sysctl -w net.ipv4.tcp_rmem="4096 12582912 16777216"
sysctl -w net.ipv4.tcp_max_syn_backlog=8096
sysctl -w net.ipv4.tcp_no_metrics_save=1 # do not cache TCP metrics for subsequent connections using the same (dst_ip, src_ip, dst_port, src_port) tuple, because it is harmful and unnecessary in modern WAN networks
sysctl -w net.ipv4.tcp_slow_start_after_idle=0 # not needed in modern networks, because it begins to aggressively reduce TCP cwnd on idle connections
sysctl -w net.ipv4.tcp_tw_reuse=1 # secure option to reuse TIME-WAIT socket on outgoing connection
sysctl -w net.ipv4.ip_local_port_range="10500 65535" # we are using ports lower than 10500 for binding deckhouse modules components
sysctl -w net.ipv4.neigh.default.gc_thresh1=16384 # fix neighbour: arp_cache: neighbor table overflow!
sysctl -w net.ipv4.neigh.default.gc_thresh2=28672
sysctl -w net.ipv4.neigh.default.gc_thresh3=32768
sysctl -w net.bridge.bridge-nf-call-iptables=1 # this parameter is needed for kube-proxy to work
sysctl -w net.bridge.bridge-nf-call-arptables=1 # this parameter is needed for kube-proxy to work
sysctl -w net.bridge.bridge-nf-call-ip6tables=1 # this parameter is needed for kube-proxy to work
sysctl -w vm.dirty_ratio=80 # enable synchronous writeback of dirty pages as late as possible
sysctl -w vm.dirty_background_ratio=5 # enable parallel writeback as early as possible
sysctl -w vm.dirty_expire_centisecs=12000 # after 12 seconds we writeback dirty pages
sysctl -w fs.file-max=1000000
sysctl -w vm.min_free_kbytes=131072 # increase the safe limit for immediate page allocations in the kernel (Jumbo Frames, different IRQ handlers)
sysctl -w kernel.numa_balancing=0 # disable the overly smart NUMA node balancer so that there are no sags. NUMA affinity is better configured in advance and differently
sysctl -w fs.inotify.max_user_watches=524288 # Increase inotify (https://github.com/guard/listen/wiki/Increasing-the-amount-of-inotify-watchers#the-technical-details)
sysctl -w fs.inotify.max_user_instances=5120
sysctl -w kernel.pid_max=2000000
{{- if eq .bundle "centos" }}
sysctl -w fs.may_detach_mounts=1 # For Centos to avoid problems with unmount when container stops # https://bugzilla.redhat.com/show_bug.cgi?id=1441737
{{- end }}
# kubelet parameters
sysctl -w vm.overcommit_memory=1
sysctl -w kernel.panic_on_oops=1

{{- $fencingTime := 10 }} 
{{- if eq (dig "fencing" "mode" "" .nodeGroup) "Watchdog" }}
  {{- $fencingTime = 0 }}
{{- end }}
sysctl -w kernel.panic={{ $fencingTime }}
# we use tee for work with globs
echo 256 | tee /sys/block/*/queue/nr_requests >/dev/null # put more in the request queue, increase throughput
echo 256 | tee /sys/block/*/queue/read_ahead_kb >/dev/null # the most controversial thing, Netflix recommends increasing a little, but you need to test on different setups, this number looks safe
echo never | tee /sys/kernel/mm/transparent_hugepage/enabled >/dev/null
echo never | tee /sys/kernel/mm/transparent_hugepage/defrag >/dev/null
echo 0 | tee /sys/kernel/mm/transparent_hugepage/use_zero_page >/dev/null
echo 0 | tee /sys/kernel/mm/transparent_hugepage/khugepaged/defrag >/dev/null
echo 0 | tee /proc/sys/net/ipv4/conf/*/rp_filter >/dev/null # disable reverse-path filtering on all interfaces
EOF
chmod +x /opt/deckhouse/bin/sysctl-tuner

# Generate sysctl tuner unit
bb-sync-file /etc/systemd/system/sysctl-tuner.timer - sysctl-tuner-service-changed << EOF
[Unit]
Description=Sysctl Tuner timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=10min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/sysctl-tuner.service - sysctl-tuner-service-changed << EOF
[Unit]
Description=Sysctl Tuner

[Service]
EnvironmentFile=/etc/environment
ExecStart=/opt/deckhouse/bin/sysctl-tuner
EOF

systemctl stop sysctl-tuner.timer
