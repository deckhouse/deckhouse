{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash
export LANG=C
yum updateinfo
until yum install nc curl wget jq -y; do
  echo "Error installing packages"
  yum updateinfo
  sleep 10
done

for FS_NAME in $(mount -l -t xfs | awk '{ print $1 }'); do
  if command -v xfs_info >/dev/null && xfs_info $FS_NAME | grep -q ftype=0; then
     >&2 echo "XFS file system with ftype=0 was found ($FS_NAME). This may cause problems (https://www.suse.com/support/kb/doc/?id=000020068), please fix it and try again."
     exit 1
  fi
done

mkdir -p /var/lib/bashible/
