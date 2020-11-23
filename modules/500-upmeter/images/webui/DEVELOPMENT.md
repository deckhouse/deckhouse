## Local start

Чтобы запустить webui локально, нужно:

- Установленный yarn.
- Чтобы запустить dev-сервер, достаточно 2 команд:

```
$ yarn install
$ yarn run start:dev
```

- Если всё ок, то будет сообщение "Compiled successfully." и можно зайти на http://localhost:4800/
- Бэкенд тоже можно поднять локально, но нужен кластер с установленным CRD. Поэтому проще подключиться к тестовому кластеру в tf-cloud.

```
$ ssh -i ~/.ssh/tfadm-id-rsa ubuntu@95.217.82.188 -L8091:localhost:8091
$ sudo -i
# kubectl -n d8-upmeter port-forward upmeter-0 8091:8091
```
