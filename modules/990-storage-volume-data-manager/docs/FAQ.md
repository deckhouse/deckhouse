---
title: "Module storage-volume-data-manager: FAQ"
linkTitle: "Usage Scenarios"
---

## What if I don't want to use the d8 utility? What other ways are there to create and use DataExport resources?

You can create a resource through a yaml manifest; for convenience, we will use variables in this example (replace the values with your own):

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

After creating the resource, you need to extract the CA certificate from it:

```bash
kubectl -n $NAMESPACE get dataexport $DATA_EXPORT_RESOURCE_NAME  -o jsonpath='{.status.ca}' | base64 -d > ca.pem
```

Check the certificate:

```bash
openssl x509 -in ca.pem -noout -text | head
# It should look something like:
#   Issuer: CN = data-exporter-CA
#   Signature Algorithm: ecdsa-with-SHA256
```

Export the URL from the DataExport resource and verify the export:

```bash
export POD_URL=$(kubectl -n $NAMESPACE get dataexport $DATA_EXPORT_RESOURCE_NAME  -o jsonpath='{.status.url}')
echo "POD_URL: $POD_URL"
```

Next, we can connect using the following methods.

### 1. Using the certificate and key from the local kube config

Copy the keys from the config:

```bash
cat ~/.kube/config | grep "client-certificate-data" | awk '{print $2}' | base64 -d > client.crt
cat ~/.kube/config | grep "client-key-data" | awk '{print $2}' | base64 -d > client.key
```

Check the contents on the target PVC:

```bash
curl -v --cacert ca.pem ${POD_URL}api/v1/files/ --key client.key --cert client.crt
```

Example output:

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

### 2. Using token and roles

Create a ServiceAccount:

```bash
kubectl -n $NAMESPACE create serviceaccount data-exporter-test
```

Create a ClusterRole:

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

Create a token:

```bash
export TOKEN=$(kubectl create token data-exporter-test --duration=24h)
echo $TOKEN
```

Create a ClusterRoleBinding:

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

Check the contents on the target PVC:

```bash
curl -H "Authorization: Bearer $TOKEN" \
-v --cacert ca.pem ${POD_URL}api/v1/files/
```

Example output:

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

Important notes:

- Files are downloaded using standard GET requests containing the file path in the URL: GET /api/v1/files/largeimage.iso, GET /api/v1/files/directory/largeimage.iso. The file path should not end with /.
  This download method is supported by standard tools: browsers, curl, etc. File resumption is supported, but compression is not;
- Accessing a directory is carried out with a similar GET request, where the directory path should end with /: GET /api/v1/files/ - path to root, GET /api/v1/files/directory/ - path to directory;
- When accessing a directory, a file listing in this directory is provided: a JSON string containing the list of files is sent in the response body, including the name, type, and size of the files. File sizes are not cached and are recalculated on each directory request;
