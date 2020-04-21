Команда, которой можно загрузить pki с существующей тачки:
```sh
kubectl -n kube-system create secret generic d8-pki \
  --from-file=ca.crt=/etc/kubernetes/pki/ca.crt \
  --from-file=ca.key=/etc/kubernetes/pki/ca.key \
  --from-file=sa.pub=/etc/kubernetes/pki/sa.pub \
  --from-file=sa.key=/etc/kubernetes/pki/sa.key \
  --from-file=front-proxy-ca.crt=/etc/kubernetes/pki/front-proxy-ca.crt \
  --from-file=front-proxy-ca.key=/etc/kubernetes/pki/front-proxy-ca.key \
  --from-file=etcd-ca.crt=/etc/kubernetes/pki/etcd/ca.crt \
  --from-file=etcd-ca.key=/etc/kubernetes/pki/etcd/ca.key
```

Параметры:
Нужно сразу предусмотреть параметр для модуля publishAPI, в котором:
* apiserver:
  * bindToWildcard – true/false (будет слушать на 6443)
  * certSANs – дополнительные
  * loadBalancer – если указано, будет создан сервис с типом `LoadBalancer` (d8-control-plane-apiserver в ns kube-system):
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
        * **Внимание!** модуль не учитывает особенности указания аннотаций в различных облаках. Если аннотации для заказа load balancer'а применяются только при создании сервиса, то для обновления подобных параметров вам необходимо будет выключить и включить параметр `publishAPI`.
    * `sourceRanges` — список CIDR, которым разрешен доступ к API.
        * Облачный провайдер может не поддерживать данную опцию и игнорировать её.
