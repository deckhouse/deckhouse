---
title: "Модуль virtualization: FAQ"
---

## Как сохранить образ в registry

Чтобы сохранить образ в registry вам необходимо собрать docker image с одной директорией `/image` в которую следует положить образ с произвольным именем.  
Образ может быть как в формате `qcow2`, так и в формате `raw`.

Пример `Dockerfile`:

```Dockerfile
FROM scratch
ADD https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img /disk/jammy-server-cloudimg-amd64.img
```
