<script type="text/javascript" src='{% javascript_asset_tag dvp-getting-started-shared %}[_assets/js/dvp/getting-started-dvp-shared.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag dvp-getting-started-access %}[_assets/js/dvp/getting-started-dvp-access.js]{% endjavascript_asset_tag %}'></script>

## Настройка платформы

Параметры кластера, NFS, Ingress, пользователь и проект задаются в `config.yml` на шаге установки. Ниже — проверка после bootstrap.

Редакции CE и EE: [возможности Kubernetes Platform](https://deckhouse.ru/products/kubernetes-platform/features/), [редакции Virtualization Platform](https://deckhouse.ru/products/virtualization-platform/documentation/about/editions.html).

### Ingress NGINX

Убедитесь, что поды Ingress-контроллера в статусе `Running` (на master-узле):

```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```

### DNS

Добавьте DNS-записи для шаблона домена, который вы указали на шаге параметров кластера (`publicDomainTemplate`). Для wildcard-шаблона достаточно одной A-записи на публичный IP узла с Ingress; иначе создайте записи для нужных поддоменов (например `console`, `grafana`, `prometheus` и т.д. вместо `%s` в шаблоне).

### Проверка узлов

```bash
sudo -i d8 k get no
```

Все узлы должны быть `Ready`.

### NFS (`csi-nfs`)

```bash
sudo -i d8 k get module csi-nfs -w
sudo -i d8 k get nfsstorageclass
sudo -i d8 k get storageclass
```

Убедитесь, что имя StorageClass по умолчанию совпадает с NFS StorageClass из шага «Параметры кластера» и с `global.defaultClusterStorageClass` в `ModuleConfig/global` (если вы не меняли имя на шаге установки, это `nfs-storage-class`).

### Модуль `virtualization`

```bash
sudo -i d8 k get po -n d8-virtualization
```

Дождитесь статуса `Running` у подов модуля.

### Доступ к консоли

Откройте веб-интерфейс по адресу `console.<ваш_суффикс_домена>` (подставьте домен из шаблона `publicDomainTemplate` вместо `%s`) и войдите с учётной записью администратора, заданной на шаге параметров кластера.
