| Термин англоязычный | Термин русскоязычный | Примечания|
| :----------| :-------------| :------|
| CRI (container runtime interface) | Среда выполнения контейнера |  |
| Load Balancer | Балансировщик нагрузки| |
| Instance | Инстанс |  |
| Namespace  | Пространство имен |  |
| Label | Лэйбл или Метка |  |
| Annotations | Аннотации |  |
| Service | Сервис |  |
|  |  |  |


[ссылка в Фигму](https://www.figma.com/file/pJOXVgTxgBAGoOjzu0oAkJ/Deckhouse-Kubernetes-Platform?type=whiteboard&node-id=1-14)

Требуют сортировки

custom resources
cert-manager
bare-metal-сервер
SSH-ключ
label selector
priority level
master-узел
single-master
multi-master
bare metal
playbook Ansible
inventory-файл
bootstrap-скрипт
shell-скрипта
При disruption update выполняется evict подов с узла. Если какие-либо поды не удалось evict'нуть, evict повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После этого поды, которые не удалось evict'нуть, удаляются.
Ingress-ресурс
control plane
containerd
YAML-файлы
требующие disruption (например, обновление ядра, смена версии containerd, значительная смена версии kubelet и пр.) — можно выбрать ручной или автоматический режим. В случае, если разрешены автоматические disruptive-обновления, перед обновлением производится drain узла (можно отключить).
Предпочтительный вариант — сделать multimaster и поменять тип CRI!
Deckhouse обновляет узлы в master NodeGroup по одному
Дождаться перехода обновленного master-узла в Ready. Выполнить итерацию для следующего master'а.
для GPU-нод
basic auth
smoke-тестирование
cloud-провайдер и cloud provider
Дополнительные secutiry groups, которые будут присвоены созданным инстансам.
root-диск
инстанс bastion     
Internet Gateway
Список дополнительных policy actions для IAM-ролей.
Используется cluster-autoscaler'ом при планировании, только когда в NodeGroup'е еще нет узлов (при minPerZone равном 0). Если в NodeGroup уже есть узлы, cluster-autoscaler использует при планировании фактические данные (CPU, memory) о мощности узла и не использует данные параметра capacity.
Создание spot-инстансов (spot instance). Spot-инстансы будут запускаться с минимально возможной для успешного запуска ценой за час.
Service Endpoints
Параметр доступен только для layout Standard. (схему вместо layout я тоже видела)
Указание Exists равносильно допуску любого значения для value, чтобы под с указанным toleration удовлетворял соответствующему taint.
Accelerated Networking обеспечивает пропускную способность сети до 30 Гбит/с.
Идентификатор tenant'а.
label
Также будет добавлен маршрут для internal-интерфейса узла на всю подсеть, указанную в nodeNetworkCIDR.
Список DHCP-опций, которые будут установлены на все подсети.
Search-домен.
Deckhouse создаст service account c ролью monitoring.viewer и API-ключ для него. Для основного service account'а требуется роль admin. (кажется, это ресурс ServiceAccount)
Тип container runtime, используемый на узлах кластера (в NodeGroup'ах) по умолчанию.
wildcard-домен
container registry
Вызов webhook'а произойдет после появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента ее применения в кластере.
Basic-аутентификация
Bearer token
Пространство имен Service'а Alertmanager.
Список receiver'ов. Receiver определяет одну или несколько интеграций для отправки оповещений.
Структура, содержащая сертификат CA для target'ов.
Список responder'ов, ответственных за уведомления.

[Стандартизированный глоссарий для Kubernetes](https://kubernetes.io/ru/docs/reference/glossary/?fundamental=true)
