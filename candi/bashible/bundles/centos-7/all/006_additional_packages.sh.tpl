#!/bin/bash -e

### Миграция 04.03.2020: Удалить после выката на все кластеры https://github.com/deckhouse/deckhouse/merge_requests/1775
if ! rpm -q nfs-utils >/dev/null 2>/dev/null ; then
  yum install -y nfs-utils
fi
