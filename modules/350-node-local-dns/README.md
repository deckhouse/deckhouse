Модуль node-local-dns
=====================

Разворачивается кеширующий CoreDNS (доступен по IP 169.254.20.10) в DaemonSet на все ноды и настраивает хитрый fallback средствами iptables (см. подробнее Принцип работы).

Модуль состоит из оригинального CoreDNS (нет никаких изменений в основном функционале), в который добавили настройку сети и iptables (за основу был взят [node-cache](https://github.com/kubernetes/dns/blob/master/cmd/node-cache/main.go)).

Конфигурация
------------

У модуля нет никаких настроек, но по-умолчанию он **выключен** и для его использования нужно изменить настройки kubelet'ов.

Чтобы начать использовать node-local-dns необходимо:
1. Включить модуль добавив следующий конфиг:
    ```yaml
    nodeLocalDns: "{}"
    ```
2. Настроить все kubelet'ы в кластере на использование нового DNS через опцию `--cluster-dns=169.254.20.10`.

Прицип работы
------------

## Запуск

1. Настраивает цепочки и правила iptables;
2. Настраивает сетевой интерфейс типа dummy и назначает ему link-local IPv4 адрес (169.254.20.10);
3. Запускает со-процесс, который каждые 60 секунд проверяет наличие iptables цепочек/правил и интерфейса с IPv4 адресом;
4. Запускает CoreDNS.

## Завершение

1. Останавливает CoreDNS;
2. Останавливает со-процесс;
3. Удаляет сетевой интерфейс.

## Подробно о фазах

### iptables

Создаёт цепочку `NODE-LOCAL-DNS` и 5 (или 6) правил. Всё в таблице `nat`.
В зависимости от режима работы kube-proxy генерирует различный набор iptables правил для fallback'а резолвов в локальных контейнерах на кластерный kube-dns.

##### kubeProxyMode: ipvs

1. `-A PREROUTING -d 169.254.20.10/32 -p tcp -m tcp --dport 53 -j NODE-LOCAL-DNS`
2. `-A PREROUTING -d 169.254.20.10/32 -p udp -m udp --dport 53 -j NODE-LOCAL-DNS`
3. `-A NODE-LOCAL-DNS -m socket -j RETURN`
4. `-A NODE-LOCAL-DNS -d 169.254.20.10/32 -p tcp -m tcp --dport 53 -j DNAT --to-destination 192.168.0.10:53`
5. `-A NODE-LOCAL-DNS -d 169.254.20.10/32 -p udp -m udp --dport 53 -j DNAT --to-destination 192.168.0.10:53`

* 1 и 2 правила отправляет пакет на обработку в цепочку `NODE-LOCAL-DNS`;
* 3 правило выполняет проверку наличия *слушающего* сокета для входящего пакета. Таким обращом, когда компонент CoreDNS из node-local-dns запустится, правило начнёт отрабатывать и возвращать пакет обратно, не доходят до DNAT правил, отправляющих пакет в serviceIP кластерного DNS.
* 4 и 5 правила выполняют DNAT пакета к serviceIP кластерного DNS.

##### kubeProxyMode: iptables

1. `-A PREROUTING -d 169.254.20.10/32 -p tcp -m tcp --dport 53 -j NODE-LOCAL-DNS`
2. `-A PREROUTING -d 169.254.20.10/32 -p udp -m udp --dport 53 -j NODE-LOCAL-DNS`
3. `-A NODE-LOCAL-DNS -m socket -j RETURN`
4. `-A NODE-LOCAL-DNS -d 169.254.20.10/32 -j KUBE-MARK-MASQ`
5. `-A NODE-LOCAL-DNS -d 169.254.20.10/32 -p tcp -j KUBE-SVC-ERIFXISQEP7F7OF4`
6. `-A NODE-LOCAL-DNS -d 169.254.20.10/32 -p udp -j KUBE-SVC-TCOU7JCQXEZGVUNU`

* 1 и 2 правила отправляет пакет на обработку в цепочку `NODE-LOCAL-DNS`;
* 3 правило выполняет проверку наличия *слушающего* сокета для входящего пакета. Таким обращом, когда компонент CoreDNS из node-local-dns запустится, правило начнёт отрабатывать и возвращать пакет обратно, не доходят до DNAT правил, отправляющих пакет в serviceIP кластерного DNS.
* 4 правило навешивает метку, по которой в POSTROUTING происходит MASQUERADE;
* 5 и 6 правила отправляют connection в цепочки, которые сгенерировал kube-proxy для kube-dns Service.
    * **Важное замечание.** Имя цепочек генерируется из сервиса в [таком](https://github.com/kubernetes/kubernetes/blob/master/pkg/proxy/iptables/proxier.go#L558-L573) формате (для kube-dns): kube-system/kube-dns:dnsudp. Т.к. на всех наших кластерах kube-dns сервис одинаков, никакой логики выбора имени цепочки не предусмотрено.

#### node-local-dns запущен (kubeProxyMode: ipvs)

![alive](doc/alive-node-local-dns.png)

#### node-local-dns не запущен (kubeProxyMode: ipvs)

![dead](doc/dead-node-local-dns.png)

### Networking

1. Создаёт интерфейс с типом dummy, не связанный ни с какими физически интерфейсами;
2. Присваивает ему link-local IPv4 адрес (сейчас статический 169.254.20.10/32).

### CoreDNS

Запускается с mounted конфигом.

1. DNS запросы, содержашие cluster domain (по-умолчанию, `cluster.local`) и PTR записи, forward'ит в ClusterIP кластерного DNS;
2. Все остальные запросы резолвит напрямую через рекурсивные DNS сервера, указанные в resolv.conf хоста;
3. Кэширует все запросы.
