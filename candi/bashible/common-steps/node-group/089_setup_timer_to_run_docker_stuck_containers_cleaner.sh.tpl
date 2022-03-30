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

{{- if eq .cri "Docker" }}
bb-event-on 'docker-stuck-containers-cleaner-changed' '_on_docker_stuck_containers_cleaner_changed'
_on_docker_stuck_containers_cleaner_changed() {
{{ if ne .runType "ImageBuilding" }}
  systemctl daemon-reload
  systemctl restart docker-stuck-containers-cleaner.timer
  systemctl restart docker-stuck-containers-cleaner
{{ end }}
  systemctl enable docker-stuck-containers-cleaner.timer
  systemctl enable docker-stuck-containers-cleaner
}

bb-sync-file /var/lib/bashible/docker-stuck-containers-cleaner.sh - << "EOF"
#!/bin/bash

# '^(dev-)?registry.deckhouse.io.*' is default value
# current repo pass through --image-pattern argument
image_pattern="^(dev-)?registry.deckhouse.io.*"
failed_container_kills_times=5
log_period_minutes=20

# $@ - messages
function echo_err() {
  echo "$@" 1>&2;
}

function get_stuck_containers() {
  sed_pattern='s/^.+Container ([a-z0-9]{64}) failed to exit within [0-9]+ seconds of signal [0-9]+ - using the force.+$/\1/p'
  journalctl --since "$log_period_minutes min ago" -u docker.service | \
    sed -nr "$sed_pattern" | \
    sort | \
    uniq -c | \
    awk '{ if (int($1)>='"$failed_container_kills_times"') print $2 }'
}

function parse_arguments() {
  while [[ $# -gt 0 ]]; do
      key="$1"
      case $key in
        -p|--image-pattern)
          image_pattern="$2"
          shift # past argument
          shift # past value
          ;;
        -c|--failed-kills-count-in-log)
          failed_container_kills_times="$2"
          shift # past argument
          shift # past value
          ;;
        -t|--log-period-minutes)
          log_period_minutes="$2"
          shift # past argument
          shift # past value
          ;;
      esac
    done
}

function main() {
  parse_arguments "$@"

  for container_id in $(get_stuck_containers); do
    if ! image_sha="$(docker container inspect "$container_id" -f '{{`{{.Config.Image}}`}}' 2> /dev/null)"; then
      echo_err "Container $container_id was not found or it was removed on previous run"
      continue
    fi

    image="$(docker image inspect "$image_sha" -f '{{`{{index .RepoTags 0}}`}}')"
    echo "Container $container_id has image $image"

    if ! [[ "$image" =~ $image_pattern ]]; then
      echo_err "Container $container_id with image $image does not match with pattern $image_pattern"
      continue
    fi

    if out=$(docker rm -f "$container_id"); then
      echo "Container $container_id was deleted"
    else
      echo_err "Container $container_id was not deleted. Exit code: $?. Output: $out"
    fi
  done
}

main "$@"

EOF

chmod +x /var/lib/bashible/docker-stuck-containers-cleaner.sh

bb-sync-file /etc/systemd/system/docker-stuck-containers-cleaner.timer - docker-stuck-containers-cleaner-changed << "EOF"
[Unit]
Description=Docker stuck containers cleaner timer

[Timer]
OnBootSec=20min
OnUnitActiveSec=20min

[Install]
WantedBy=multi-user.target
EOF

bb-sync-file /etc/systemd/system/docker-stuck-containers-cleaner.service - docker-stuck-containers-cleaner-changed << "EOF"
[Unit]
Description=Docker stuck containers cleaner service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/docker-stuck-containers-cleaner.sh --image-pattern '^{{ .registry.address }}{{ .registry.path }}.*' --failed-kills-count-in-log 5 --log-period-minutes 20

[Install]
WantedBy=multi-user.target
EOF
{{- else }}
# here handle switch from docker to containerd
# we need to remove cleaner

{{- if ne .runType "ImageBuilding" }}

if [[ -f "/etc/systemd/system/docker-stuck-containers-cleaner.service" ]]; then
  systemctl stop docker-stuck-containers-cleaner.service
  systemctl disable docker-stuck-containers-cleaner.service
  rm -f /etc/systemd/system/docker-stuck-containers-cleaner.service
  systemctl daemon-reload
  systemctl reset-failed
fi

if [[ -f "/etc/systemd/system/docker-stuck-containers-cleaner.timer" ]]; then
  systemctl stop docker-stuck-containers-cleaner.timer
  systemctl disable docker-stuck-containers-cleaner.timer
  rm -f /etc/systemd/system/docker-stuck-containers-cleaner.timer
fi

{{- end }}

{{- end }}
