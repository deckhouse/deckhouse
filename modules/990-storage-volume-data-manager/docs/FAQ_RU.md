---
title: "Модуль storage-volume-data-manager: FAQ"
linkTitle: "Сценарии использования"
---

## Что если я не хочу использовать утилиту d8? Каким ещё способом можно создавать и пользоваться ресурсами DataExport?

Создать ресурс можно через yaml-манифест, в примере для удобства будем использованть переменные (замените значения на нужные вам)

```bash
export NAMESPACE="d8-storage-volume-data-manager"
export DATA_EXPORT_RESOURCE_NAME="example-dataexport"
export TARGET_TYPE="PersistentVolumeClaim"
export TARGET_NAME="fs-pvc-data-exporter-fs-0"
```

```bash
k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: DataExport
metadata:
  name: ${DATA_EXPORT_RESOURCE_NAME}
  namespace: ${NAMESPACE}
spec:
  ttl: 10h
  targetRef:
    kind: ${TARGET_TYPE}
    name: ${TARGET_NAME}
EOF
```

После создания ресурса нужно взять из него ca-сертификат:

```bash
kubectl -n $NAMESPACE get dataexport $DATA_EXPORT_RESOURCE_NAME  -o jsonpath='{.status.ca}' | base64 -d > ca.pem
```

Проверяем сертификат:

```bash
openssl x509 -in ca.pem -noout -text | head
# Должно быть что-то вроде:
#   Issuer: CN = data-exporter-CA
#   Signature Algorithm: ecdsa-with-SHA256
```

Экспортируем URL-адрес из DataExport ресурса и проверяем экспорт:

```bash
export POD_URL=$(kubectl -n $NAMESPACE get dataexport $DATA_EXPORT_RESOURCE_NAME  -o jsonpath='{.status.url}')
echo "POD_URL: $POD_URL"
```

Далее, мы можем подключаться следующими методами.

### 1. С указанием сертификата и ключа из локального kube config

Копируем ключи из конфига:

```bash
cat ~/.kube/config | grep "client-certificate-data" | awk '{print $2}' | base64 -d > client.crt
cat ~/.kube/config | grep "client-key-data" | awk '{print $2}' | base64 -d > client.key
```

Проверяем содержимое на целевой PVC:

```bash
curl -v --cacert ca.pem ${POD_URL}api/v1/files/ --key client.key --cert client.crt
```

Пример вывода:

```bash
..
..

< 
* TLSv1.2 (IN), TLS header, Supplemental data (23):
{"apiVersion": "v1", "items": [{"name":"4.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"hello","size":5,"modTime":"2025-03-03 10:53:06.895434814 +0000 UTC","type":"file"}
,{"name":"7.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"lost+found","modTime":"2025-03-03 10:29:31 +0000 UTC","type":"dir"}
,{"name":"8.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"10.txt","size":13,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"9.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"3.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"2.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"1.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"6.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"5.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
]}
```

### 2. С использованием токена и ролей

Создаём ServiceAccount:

```bash
kubectl -n $NAMESPACE create serviceaccount data-exporter-test
```

Создаём ClusterRole

```bash
kubectl create -f - <<EOF
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
name: data-exporter-test-role
rules:
- apiGroups: ["storage.deckhouse.io"]
  resources: ["dataexports/download"]
  verbs: ["create"]
EOF
```

Создаём токен

```bash
export TOKEN=$(kubectl create token data-exporter-test --duration=24h)
echo $TOKEN
```

Создаём ClusterRoleBinding:

```bash
kubectl create -f - <<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
name: data-exporter-test-role-binding
namespace: ${NAMESPACE}
subjects:
- kind: ServiceAccount
  name: data-exporter-test
  namespace: ${NAMESPACE}
  roleRef:
  kind: ClusterRole
  name: data-exporter-test-role
  apiGroup: rbac.authorization.k8s.io
  EOF
```

Проверяем содержимое на целевой PVC:

```bash
curl -H "Authorization: Bearer $TOKEN" \
-v --cacert ca.pem ${POD_URL}api/v1/files/
```

Пример вывода:

```bash
..
..

< 
* TLSv1.2 (IN), TLS header, Supplemental data (23):
{"apiVersion": "v1", "items": [{"name":"4.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"hello","size":5,"modTime":"2025-03-03 10:53:06.895434814 +0000 UTC","type":"file"}
,{"name":"7.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"lost+found","modTime":"2025-03-03 10:29:31 +0000 UTC","type":"dir"}
,{"name":"8.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"10.txt","size":13,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"9.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"3.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"2.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"1.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"6.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
,{"name":"5.txt","size":12,"modTime":"2025-04-01 08:02:00.228156524 +0000 UTC","type":"file"}
]}
```

Важные примечания:

- Файлы скачиваются при помощи стандартных GET-запросов, содержащих в URL путь к файлу: GET /api/v1/files/largeimage.iso, GET /api/v1/files/directory/largeimage.iso. Путь к файлу не должен заканчиваться символом /.
Такой метод скачивания поддерживается стандартными средствами: браузерами, curl и т.д. Поддерживается докачка файлов, при этом сжатие не поддерживается;
- Обращение к директории осуществляется аналогичным GET-запросом, при этом путь к директории должен заканчиваться символом /: GET /api/v1/files/ - путь к root, GET /api/v1/files/directory/ - путь к directory;
- При обращении к директории обеспечивается листинг файлов в этой директории: в теле ответа отправляется JSON-строка, содержащая список файлов: имя, тип и размер. Размеры файлов не кэшируются, при каждом запросе директории вычисляются заново;
