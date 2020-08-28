---
title: Поддержка модуля istio
---


Оригинальный istio поставляется в виде helm-чарта, который мы преобразовали в наш формат выкинув много лишнего и переделав их костыли на наши.

Как обновлять istio
-------------------

* Склонировать себе [официальный git-репозиторий](https://github.com/istio/istio) и diff-нуть тег текущей версии со свежим тегом:
```shell
git clone https://github.com/istio/istio.git --tags
cd istio
git diff 1.1.7 1.1.8
```
Нас интересуют изменения в yaml-манифестах и если таковые есть, то вносим соответствующие изменения в наш модуль.
* Меняем версию образов в `images/*/Dockerfile`.
* Меняем версию istio в [описании модуля](/modules/360-istio/).

Как настроили модуль первично
-----------------------------

Срендерили базовый минимальный шаблон:

```
helm template install/kubernetes/helm/istio \
    --name istio \
    --namespace istio-system \
    --set certmanager.enabled=false \
    --set galley.enabled=true \
    --set gateways.enabled=false \
    --set gateways.istio-ingressgateway.enabled=false \
    --set gateways.istio-egressgateway.enabled=false \
    --set global.controlPlaneSecurityEnabled=true \
    --set global.mtls.enabled=false \
    --set global.enableTracing=false \
    --set grafana.enabled=false \
    --set istio_cni.enabled=false \
    --set istiocoredns.enabled=false \
    --set kiali.enabled=false \
    --set mixer.enabled=true \
    --set mixer.policy.enabled=false \
    --set mixer.telemetry.enabled=false \
    --set nodeagent.enabled=false \
    --set pilot.enabled=true \
    --set pilot.autoscaleEnabled=false \
    --set prometheus.enabled=false \
    --set security.enabled=true \
    --set servicegraph.enabled=false \
    --set sidecarInjectorWebhook.enabled=true \
    --set tracing.enabled=false > base.yaml
```

Полученный yaml-файл разбили на папки этим скриптом:
```python
#!/usr/bin/env python

import re
import sys, os

content = open(sys.argv[1], 'r').read()
matches = re.findall("^(# Source: .*?)(?=(?:---\n# Source|\Z))", content, re.M | re.S)
for i in matches:
  lines = i.split('\n')

  if not len(''.join(lines[1:])):
    continue # if all strings are empty

  first_line_splitted = lines[0].split('/')

  # variants:
  # # Source: istio/charts/security/templates/tests/test-citadel-connection.yaml
  # # Source: istio/charts/galley/templates/poddisruptionbudget.yaml
  # # Source: istio/charts/pilot/templates/poddisruptionbudget.yaml

  if 'tests' in first_line_splitted:
    continue # who needs tests?

  if first_line_splitted[1] == 'charts':
    target = first_line_splitted[2] + '/' + first_line_splitted[4]

  elif first_line_splitted[1] == 'templates':
    target = first_line_splitted[2]

  target = '../templates/' + target
  os.system('mkdir -p `dirname ' + target + '`')
  f = open(target, 'w')
  f.write("\n".join(lines[1:])) # cut off first line with "# Source"
  f.close()
```

Далее меняя настройки helm-чарта сравнивали их с базовой версией и добавляли IF-чики.
