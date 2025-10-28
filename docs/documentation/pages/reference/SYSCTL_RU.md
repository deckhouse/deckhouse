---
title: Параметры sysctl, настраиваемые платформой
description: "Список параметров sysctl, которые Deckhouse Platform Certified Security Edition настраивает и поддерживает на узлах кластера."
permalink: ru/reference/sysctl.html
lang: ru
search: sysctl parameters reference, kernel parameters reference, параметры ядра, настройка системы
---

Deckhouse автоматически настраивает и управляет рядом параметров работы ядра сервера, используя утилиту `sysctl`.
Заданные параметры повышают сетевую пропускную способность, предотвращают нехватку ресурсов
и оптимизируют управление памятью.

{% alert level="info" %}
При изменении этих параметров Deckhouse автоматически вернет их к значениям, перечисленным ниже.
{% endalert %}

<table>
  <thead>
    <tr>
      <th>Параметр</th>
      <th>Значение, которое устанавливает Deckhouse</th>
      <th>Описание</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>/sys/block/*/queue/nr_requests</code></td>
      <td><code>256</code></td>
      <td>Количество запросов в очереди для блочных устройств.</td>
    </tr>
    <tr>
      <td><code>/sys/block/*/queue/read_ahead_kb</code></td>
      <td><code>256</code></td>
      <td>Объем дополнительных данных, которые ядро считывает с диска для ускорения чтения в дальнейшем.</td>
    </tr>
    <tr>
      <td><code>/sys/kernel/mm/transparent_hugepage/enabled</code></td>
      <td><code>never</code></td>
      <td>Отключает Transparent HugePage.</td>
    </tr>
    <tr>
      <td><code>/sys/kernel/mm/transparent_hugepage/defrag</code></td>
      <td><code>never</code></td>
      <td>Отключает дефрагментацию Transparent HugePage.</td>
    </tr>
    <tr>
      <td><code>/sys/kernel/mm/transparent_hugepage/use_zero_page</code></td>
      <td><code>0</code></td>
      <td>Отключает использование нулевых huge-страниц.</td>
    </tr>
    <tr>
      <td><code>/sys/kernel/mm/transparent_hugepage/khugepaged/defrag</code></td>
      <td><code>0</code></td>
      <td>Отключает дефрагментацию через <code>khugepaged</code>.</td>
    </tr>
    <tr>
      <td><code>/proc/sys/net/ipv4/conf/*/rp_filter</code></td>
      <td><code>0</code></td>
      <td>Отключает «фильтрацию обратного пути» (reverse path filtering) для всех интерфейсов.</td>
    </tr>
    <tr>
      <td><code>fs.file-max</code></td>
      <td><code>1000000</code></td>
      <td>Максимальное количество открытых файлов.</td>
    </tr>
    <tr>
      <td><code>fs.inotify.max_user_instances</code></td>
      <td><code>5120</code></td>
      <td>Максимальное количество экземпляров inotify.</td>
    </tr>
    <tr>
      <td><code>fs.inotify.max_user_watches</code></td>
      <td><code>524288</code></td>
      <td>Максимальное количество файлов, отслеживаемых одним экземпляром inotify.</td>
    </tr>
    <tr>
      <td><code>fs.may_detach_mounts</code></td>
      <td><code>1</code></td>
      <td>Разрешает отмонтировать файловую систему в режиме lazy unmounting.</td>
    </tr>
    <tr>
      <td><code>kernel.numa_balancing</code></td>
      <td><code>0</code></td>
      <td>Запрещает автоматическую балансировку памяти с архитектурой NUMA.</td>
    </tr>
    <tr>
      <td><code>kernel.panic</code></td>
      <td><code>10 (0, если включен fencing)</code></td>
      <td>Время в секундах, после которого произойдет перезагрузка узла при возникновении фатальной ошибки kernel panic. По умолчанию устанавливается значение <code>10</code>. Если на узле включен режим <a href="/modules/node-manager/cr.html#nodegroup-v1-spec-fencing"><code>fencing</code></a>, устанавливается значение <code>0</code>, тем самым предотвращая перезагрузку узла.</td>
    </tr>
    <tr>
      <td><code>kernel.panic_on_oops</code></td>
      <td><code>1</code></td>
      <td>Разрешает системе активировать kernel panic при возникновении непредвиденной ошибки oops. Параметр необходим для корректной работы kubelet.</td>
    </tr>
    <tr>
      <td><code>kernel.pid_max</code></td>
      <td><code>2000000</code></td>
      <td>Максимальное количество ID процессов (PID), которое можно назначить в системе.</td>
    </tr>
    <tr>
      <td><code>net.bridge.bridge-nf-call-arptables</code></td>
      <td><code>1</code></td>
      <td>Разрешает фильтрацию трафика с помощью arptables. Параметр необходим для корректной работы kube-proxy.</td>
    </tr>
    <tr>
      <td><code>net.bridge.bridge-nf-call-ip6tables</code></td>
      <td><code>1</code></td>
      <td>Разрешает фильтрацию трафика с помощью ip6tables. Параметр необходим для корректной работы kube-proxy.</td>
    </tr>
    <tr>
      <td><code>net.bridge.bridge-nf-call-iptables</code></td>
      <td><code>1</code></td>
      <td>Разрешает фильтрацию трафика с помощью iptables. Параметр необходим для корректной работы kube-proxy.</td>
    </tr>
    <tr>
      <td><code>net.core.netdev_max_backlog</code></td>
      <td><code>5000</code></td>
      <td>Максимальное количество пакетов в очереди на обработку.</td>
    </tr>
    <tr>
      <td><code>net.core.rmem_max</code></td>
      <td><code>16777216</code></td>
      <td>Максимальный размер приемного буфера в байтах.</td>
    </tr>
    <tr>
      <td><code>net.core.somaxconn</code></td>
      <td><code>1000</code></td>
      <td>Максимальное количество соединений в очереди.</td>
    </tr>
    <tr>
      <td><code>net.core.wmem_max</code></td>
      <td><code>16777216</code></td>
      <td>Максимальный размер пересылочного буфера в байтах.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.conf.all.forwarding</code></td>
      <td><code>1</code></td>
      <td>Разрешает перенаправление IPv4-пакетов между сетевыми интерфейсами. Равнозначно параметру <code>net.ipv4.ip_forward</code>.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.ip_local_port_range</code></td>
      <td><code>"32768 61000"</code></td>
      <td>Диапазон портов для исходящих TCP- и UDP-соединений.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.neigh.default.gc_thresh1</code></td>
      <td><code>16384</code></td>
      <td>Нижний порог количества ARP-записей, после которого система начнет удалять старые записи.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.neigh.default.gc_thresh2</code></td>
      <td><code>28672</code></td>
      <td>Средний порог количества ARP-записей, после которого система запустит очистку памяти.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.neigh.default.gc_thresh3</code></td>
      <td><code>32768</code></td>
      <td>Предельно допустимое количество ARP-записей.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.tcp_max_syn_backlog</code></td>
      <td><code>8096</code></td>
      <td>Максимальное количество SYN-соединений в очереди.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.tcp_no_metrics_save</code></td>
      <td><code>1</code></td>
      <td>Запрещает сохранение TCP-метрик закрытых соединений и их повторное использование для новых соединений.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.tcp_rmem</code></td>
      <td><code>"4096 12582912 16777216"</code></td>
      <td>Размеры приемного буфера для входящих TCP-пакетов в байтах: <code>"<минимальный> <по умолчанию> <максимальный>"</code>.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.tcp_slow_start_after_idle</code></td>
      <td><code>0</code></td>
      <td>Запрещает использование окна перезагрузки (congestion window, CWND) и алгоритма медленного старта для TCP-соединений.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.tcp_tw_reuse</code></td>
      <td><code>1</code></td>
      <td>Разрешает повторное использование исходящих TCP-соединений в состоянии <code>TIME-WAIT</code>.</td>
    </tr>
    <tr>
      <td><code>net.ipv4.tcp_wmem</code></td>
      <td><code>"4096 12582912 16777216"</code></td>
      <td>Размеры пересылочного буфера для исходящих TCP-пакетов в байтах: <code>"<минимальный> <по умолчанию> <максимальный>"</code>.</td>
    </tr>
    <tr>
      <td><code>net.netfilter.nf_conntrack_max</code></td>
      <td><code><кол-во ядер * 131072> или 524288</code></td>
      <td>Максимальное количество отслеживаемых соединений в таблице conntrack. Рассчитывается по формуле: «количество выделенных CPU-ядер» * 131072, но не менее <code>524288</code>.</td>
    </tr>
    <tr>
      <td><code>net.nf_conntrack_max</code></td>
      <td><code><кол-во ядер * 131072> или 524288</code></td>
      <td>Максимальное количество отслеживаемых соединений в таблице conntrack для старых версий ядра. Рассчитывается по формуле: «количество выделенных CPU-ядер» * 131072, но не менее <code>524288</code>.</td>
    </tr>
    <tr>
      <td><code>vm.dirty_background_ratio</code></td>
      <td><code>5</code></td>
      <td>Доля системной памяти в процентах, которую допустимо занимать "грязными" страницами (dirty pages), прежде чем начнется запись на диск в асинхронном режиме.</td>
    </tr>
    <tr>
      <td><code>vm.dirty_expire_centisecs</code></td>
      <td><code>12000</code></td>
      <td>Продолжительность периода в сотых долях секунды, пока "грязная" страница (dirty page) может оставаться в системной памяти, после чего она должна быть записана на диск.</td>
    </tr>
    <tr>
      <td><code>vm.dirty_ratio</code></td>
      <td><code>80</code></td>
      <td>Доля системной памяти в процентах, которую допустимо занимать "грязными" страницами (dirty pages), прежде чем все процессы остановятся и будет выполнен сброс данных на диск.</td>
    </tr>
    <tr>
      <td><code>vm.min_free_kbytes</code></td>
      <td><code>131072</code></td>
      <td>Минимальный объем свободной памяти в килобайтах, который резервируется ядром для выполнения критических операций.</td>
    </tr>
    <tr>
      <td><code>vm.overcommit_memory</code></td>
      <td><code>1</code></td>
      <td>Разрешает избыточное выделение памяти (memory overcommitment).</td>
    </tr>
    <tr>
      <td><code>vm.swappiness</code></td>
      <td><code>0</code></td>
      <td>Запрещает использование файла подкачки.</td>
    </tr>
  </tbody>
</table>
