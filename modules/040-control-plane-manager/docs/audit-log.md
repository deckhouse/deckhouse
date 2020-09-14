---
title: Включение аудита событий
---

Если вам требуется вести учёт операций в кластере или отдебажить неожиданное поведение - для всего этого в Kubernetes предусмотрен [Auditing](https://kubernetes.io/docs/tasks/debug-application-cluster/debug-cluster/), настраиваемый через указание соответствующих [Audit Policy](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy). 

На данный момент модуль использует фиксированные параметры ротации логов:
```bash
--audit-log-maxage=7
--audit-log-maxbackup=10
--audit-log-maxsize=100
```
Предполагается наличие на мастер-нодах `скрейпера логов` *(filebeat, promtail)*, который будет следить за директорией с логами:
```bash
/var/log/kube-audit/audit.log
```
В зависимости от настроек `Policy` и количества запросов к **apiserver** - логов может быть очень много, соответственно глубина хранения может быть менее 30 минут. Максимальное занимаемое место на диске ограничено `1000 Мб`. Логи старше `7 дней` так же будут удалены.

## Предостережение
> ⚠️ Текущая реализация функционала не является безопасной, с точки зрения, возможности временно сломать **control-plane**.
>
> Если в секрете с конфигурационным файлом окажутся неподдерживаемые опции или опечатка, то **apiserver** не сможет запуститься.  

В случае возникновения проблем с запуском **apiserver** потребуется вручную отключить параметры `--audit-log-*` в манифесте `/etc/kubernetes/manifests/kube-apiserver.yaml` и перезапустить **apiserver** следующей командой: 
```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
```
После перезапуска у вас будет достаточно времени исправить `Secret` или [удалить его](#полезные-команды). 

## Включение и настройка
За включение отвечает параметр в `ConfigMap` `d8-system/deckhouse`
```yaml
  controlPlaneManager: |
    apiserver:
      auditPolicyEnabled: "true"
```
Конфигурация параметров осуществляется через `Secret` `kube-system/audit-policy`, внутрь которого потребуется положить `yaml` файл, закодированный `base64`:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: <base64>
```
Минимальный рабочий пример `audit-policy.yaml` выглядит так:
```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
  omitStages:
  - RequestReceived
```
Подробную информацию по настройке содержимого `audit-policy.yaml` можно получить по следующим ссылкам:
- [В официальной документации Kubernetes](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy) 
- [В нашей статье на Habr](https://habr.com/ru/company/flant/blog/468679/)
- [Опираясь на код скрипта-генератора, используемого в GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862)

## Полезные команды
Создать `Secret` из файла можно командой:
```bash
kubectl -n kube-system create secret generic audit-policy --from-file=./audit-policy.yaml
```
Удалить `Secret` из кластера:
```bash
kubectl -n kube-system delete secret audit-policy
```
