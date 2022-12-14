# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

# TODO remove after 1.42 release !!!

if ! rpm -q --quiet yum-utils; then
  yum install -y yum-utils
fi

repos="base kernels updates"
proxy=""
proxy_username=""
proxy_password=""

yum-config-manager --save --setopt=proxy=${proxy} ${repos}
yum-config-manager --save --setopt=proxy_username=${proxy_username} ${repos}
yum-config-manager --save --setopt=proxy_password=${proxy_password} ${repos}
