![resources](/images/gs/cloud-provider-openstack/openstack-standard.png)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

Создаётся внутренняя сеть кластера со шлюзом в публичную сеть, узлы не имеют публичных IP-адресов. Для master-узла заказывается floating IP.

> **Внимание!**
> Если провайдер не поддерживает SecurityGroups, то все приложения, запущенные на узлах с floating IP, будут доступны по белому IP.
Например, kube-apiserver на мастерах будет доступен по 6443 порту. Чтобы избежать этого, рекомендуется использовать схему размещения SimpleWithInternalNetwork.
