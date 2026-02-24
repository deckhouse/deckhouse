// Связи от акторов
admin               -> dkp "Устанавливает и настраивает платформу"
developer           -> dkp "Использует платформу"
user                -> dkp "Использует запущенные приложения"
security-engineer   -> dkp "Управляет информационной безопасностью"

// Связи между подсистемами
dkp.cluster-and-infrastructure  -> dkp.kubernetes-and-scheduling
dkp.deckhouse-subsystem         -> dkp.kubernetes-and-scheduling
dkp.delivery                    -> dkp.kubernetes-and-scheduling
dkp.iam                         -> dkp.kubernetes-and-scheduling
dkp.kubernetes-and-scheduling   -> dkp.deckhouse-subsystem
dkp.kubernetes-and-scheduling   -> dkp.network
dkp.managed-services            -> dkp.kubernetes-and-scheduling
dkp.network                     -> dkp.kubernetes-and-scheduling
dkp.network                     -> dkp.deckhouse-subsystem
dkp.network                     -> dkp.iam
dkp.observability-subsystem     -> dkp.cluster-and-infrastructure
dkp.observability-subsystem     -> dkp.deckhouse-subsystem
dkp.observability-subsystem     -> dkp.iam
dkp.observability-subsystem     -> dkp.kubernetes-and-scheduling
dkp.observability-subsystem     -> dkp.managed-services
dkp.observability-subsystem     -> dkp.network
dkp.observability-subsystem     -> dkp.security
dkp.observability-subsystem     -> dkp.storage
dkp.observability-subsystem     -> dkp.delivery
dkp.observability-subsystem     -> dkp.virtualization-subsystem
dkp.security                    -> dkp.kubernetes-and-scheduling
dkp.storage                     -> dkp.kubernetes-and-scheduling
dkp.virtualization-subsystem    -> dkp.kubernetes-and-scheduling

// Связи в подсистеме Cluster & Infrastructure
dkp.cluster-and-infrastructure  -> ntp-servers "Синхронизирует время"
dkp.cluster-and-infrastructure  -> iaas "Управляет ресурсами IaaS"
dkp.chrony                      -> ntp-servers "Синхронизирует время"
dkp.node-manager                -> dkp.bashible
dkp.node-manager                -> dkp.cluster-control-plane
dkp.node-manager                -> dkp.terraform-manager
dkp.terraform-manager           -> iaas "Управляет ресурсами IaaS постоянно (dhctl converge-periodical)"
dkp.cloud-providers             -> iaas "Управляет ресурсами IaaS"
dkp.cloud-provider              -> iaas "Управляет ресурсами IaaS"

// Связи в модуле node-manager
dkp.bashible                -> dkp.kube-apiserver "Скачивает секреты и ресурсы bashible со скриптами [6443 TCP]"
dkp.capi-controller-manager -> dkp.kube-apiserver "Следит за Custom Resources [6443 TCP]"
dkp.infrastructure-provider -> dkp.kube-apiserver "Следит за Custom Resources [6443 TCP]"
dkp.caps-controller-manager -> dkp.kube-apiserver "Следит за Custom Resources [6443 TCP]"
dkp.caps-controller-manager -> iaas "Управляет статическими узлами (ограниченно, без заказа узлов)"
dkp.cluster-autoscaler      -> dkp.kube-apiserver "Следит за нагрузкой на узлах, выполняет автомасштабирование узлов [6443 TCP]"
dkp.kube-apiserver          -> dkp.capi-controller-manager "Validating/mutating вебхуки на capi-webhook-service [443:4200 TCP]"
dkp.kube-apiserver          -> dkp.bashible-apiserver "Пересылает запросы на ресурсы bashible через svc [443 TCP]"
dkp.prometheus-main         -> dkp.capi-kube-rbac-proxy "Собирает метрики [4211 TCP]"
dkp.capi-kube-rbac-proxy    -> dkp.kube-apiserver "Выполняет авторизацию запросов [6443 TCP]"
dkp.capi-kube-rbac-proxy    -> dkp.capi-controller-manager "Пересылает авторизованные запросы на метрики [4211 TCP]"
dkp.psi-monitor             -> dkp.proc-files "Читает данные по загрузке процессов на хосте"
dkp.prometheus-main         -> dkp.oom-kube-rbac-proxy "Собирает метрики [8443 TCP]"
dkp.oom-kube-rbac-proxy     -> dkp.kube-apiserver "Выполняет авторизацию запросов [6443 TCP]"
dkp.oom-kube-rbac-proxy     -> dkp.psi-monitor "Пересылает авторизованные запросы на метрики [8080 TCP]"
dkp.prometheus-main         -> dkp.ca-kube-rbac-proxy "Собирает метрики [8443 TCP]"
dkp.ca-kube-rbac-proxy      -> dkp.kube-apiserver "Выполняет авторизацию запросов [6443 TCP]"
dkp.ca-kube-rbac-proxy      -> dkp.cluster-autoscaler "Пересылает авторизованные запросы на метрики [8085 TCP]"
dkp.fencing-agent           -> dkp.kube-apiserver "Проверяет доступность Kube-API [6443 TCP]"
dkp.fencing-controller      -> dkp.kube-apiserver "Удаляет ресурс Node для недоступного узла [6443 TCP]"
dkp.fencing-agent           -> dkp.watchdog-file "Пишет в устройство на хосте"
dkp.cloud-provider          -> dkp.kube-apiserver "Следит за Custom Resources [6443 TCP]"
dkp.csi-driver              -> iaas "Управляет дисками"
dkp.cloud-controller-manager -> iaas "Управляет балансировщиками и прочими ресурсами IaaS"
dkp.csi-driver              -> dkp.kube-apiserver "Следит за PV"
dkp.cloud-controller-manager -> dkp.kube-apiserver "Следит за ресурсами Loadbalancer, Node и тд"
user                        -> dhctl "Управляет узлами через утилиту dhctl (bootstrap, converge)"
dhctl                       -> iaas "Управляет ресурсами IaaS по требованию"
dhctl                       -> dkp.kube-apiserver "Читает секрет с состоянием Terraform [6443 TCP]"
dkp.terraform-manager       -> dkp.kube-apiserver "Читает секрет с состоянием Terraform [6443 TCP]"
user-static-node            -> iaas "Работает со статическими узлами"
dkp.infrastructure-provider -> iaas "Управляет виртуальными машинами"

// Связи в модуле terraform-manager
dkp.terraform-auto-converger-to-tofu-migrator   -> dkp.kube-apiserver "Читает/сохраняет секрет с состоянием Terraform [6443 TCP]"
dkp.terraform-auto-converger                    -> dkp.kube-apiserver "Читает секрет с состоянием Terraform [6443 TCP]"
dkp.terraform-state-exporter                    -> dkp.kube-apiserver "Читает секрет с состоянием Terraform [6443 TCP]"
dkp.terraform-auto-converger-kube-rbac-proxy    -> dkp.kube-apiserver "Выполняет авторизацию запросов [6443 TCP]"
dkp.terraform-state-exporter-kube-rbac-proxy    -> dkp.kube-apiserver "Выполняет авторизацию запросов [6443 TCP]"
dkp.prometheus-main                             -> dkp.terraform-auto-converger-kube-rbac-proxy "Собирает метрики [9100 TCP]"
dkp.terraform-auto-converger-kube-rbac-proxy    -> dkp.terraform-auto-converger "Пересылает авторизованные запросы на метрики [9101 TCP]"
dkp.prometheus-main                             -> dkp.terraform-state-exporter-kube-rbac-proxy "Собирает метрики [9100 TCP]"
dkp.terraform-state-exporter-kube-rbac-proxy    -> dkp.terraform-state-exporter "Пересылает авторизованные запросы на метрики [9101 TCP]"
dkp.terraform-auto-converger                    -> iaas "Управляет ресурсами IaaS постоянно (dhctl converge-periodical)"

// Связи в подсистеме Deckhouse
dkp.deckhouse-subsystem -> registry-external "Скачивает образы"
dkp.console             -> dkp.cluster-control-plane
dkp.dashboard           -> dkp.cluster-control-plane
dkp.deckhouse           -> dkp.cluster-control-plane
dkp.registry            -> registry-external "Скачивает образы"

// Связи в подсистеме Delivery
dkp.pod-reloader -> dkp.cluster-control-plane

// Связи в подсистеме IAM
dkp.iam                         -> authn-providers "Выполняет аутентификацию пользователей"
dkp.multitenancy-manager        -> dkp.cluster-control-plane
dkp.namespace-configurator      -> dkp.cluster-control-plane
dkp.user-authn                  -> authn-providers "Выполняет аутентификацию пользователей"
dkp.user-authn                  -> dkp.cluster-control-plane

// Связи в модуле user-authn
dkp.console-dex-authenticator           -> dkp.dex "4. Пересылает запросы на аутентификацию [443 TCP]"
dkp.user-app-dex-authenticator          -> dkp.dex "4. Пересылает запросы на аутентификацию [443 TCP]"
dkp.user-app-with-dex-client            -> dkp.dex "4. Выполняет аутентификацию [443 TCP]"
dkp.user-app-dex-self-signed-generator  -> dkp.user-app-dex-authenticator "Передает сертификаты через EmptyDir том"
dkp.dex-kube-rbac-proxy                 -> dkp.kube-apiserver "Выполняет авторизацию запросов"
dkp.dex-kube-rbac-proxy                 -> dkp.dex "Пересылает авторизованные запросы на метрики [5558 TCP]"
dkp.dex                                 -> authn-providers "5. Выполняет аутентификацию пользователей"
dkp.dex                                 -> dkp.cert-manager
// dkp.kubeconfig-generator                 -> dkp.dex "Пересылает запросы на аутентификацию [443 TCP]" // Deprecated maksim.nabokikh@flant.com
dkp.user-app-dex-authenticator          -> dkp.user-app-dex-redis "Записывает и читает ID Token"
dkp.prometheus-main                     -> dkp.dex-kube-rbac-proxy "Собирает метрики [5559 TCP]"

// Связи в подсистеме Kubernetes & Scheduling
dkp.control-plane-manager   -> dkp.cluster-control-plane
dkp.descheduler             -> dkp.cluster-control-plane
dkp.kubelet                 -> dkp.containerd
// dkp.kubelet                 -> dkp.kubernetes-api-proxy
dkp.kubelet                 -> dkp.registry
dkp.vertical-pod-autoscaler -> dkp.cluster-control-plane

// Связи в модуле control-plane-manager
dkp.cpm-control-plane-manager       -> dkp.kube-apiserver "Управляет control plane'ом кластера [6443 TCP]"
dkp.kubernetes-api-proxy-reloader   -> dkp.kubernetes-api-proxy "Перезагружает kubernetes-api-proxy при изменении конфигурации"
dkp.etcd-backup                     -> dkp.etcd "Подключается для резервного копирования базы данных etcd [2379 TCP]"
dkp.etcd-backup                     -> dkp.etcd-backup-files "Сохраняет backup etcd на узле (HostPath)"
dkp.kubernetes-api-proxy            -> dkp.kube-apiserver "Перенаправляет запросы на IP узла [6443 TCP]"
dkp.kube-apiserver-healthcheck      -> dkp.kube-apiserver "Пересылает healthcheck на kubeapi-server [6443 TCP]"
dkp.kube-controller-manager         -> dkp.kube-apiserver "Отправляет запросы в Kube-API [6443]"
dkp.kube-scheduler                  -> dkp.kube-apiserver "Отправляет запросы в Kube-API [6443]"
dkp.kube-apiserver                  -> dkp.etcd "Отправляет запросы в базу данных"
dkp.kube-apiserver                  -> dkp.kubelet "Обработка команд kubectl logs, kubectl exec, kubectl port-forward [10250 TCP]"
dkp.kubelet                         -> dkp.kubernetes-api-proxy "Подключается к Kube-API через 127.0.0.1:6445"
dkp.kubelet                         -> dkp.kube-apiserver-healthcheck "Проверяет состояние kubeapi-server [3990 TCP]"
dkp.prometheus-main                 -> dkp.kube-apiserver "Собирает метрики [6443 TCP]"
dkp.prometheus-main                 -> dkp.kubelet "Собирает метрики [10250 TCP]"
dkp.prometheus-main                 -> dkp.cpl-kube-rbac-proxy "Собирает метрики [4209 TCP]"
dkp.cpl-kube-rbac-proxy             -> dkp.kube-apiserver "Выполняет авторизацию запросов"
dkp.cpl-kube-rbac-proxy             -> dkp.etcd "Пересылает авторизованные запросы на метрики [2381 TCP]"
dkp.cpl-kube-rbac-proxy             -> dkp.kube-controller-manager "Пересылает авторизованные запросы на метрики [10257 TCP]"
dkp.cpl-kube-rbac-proxy             -> dkp.kube-scheduler "Пересылает авторизованные запросы на метрики [10259 TCP]"

// Связи в подсистеме Network
network-load-balancer       -> dkp.network "Балансирует HTTP/HTTPS запросы"
dkp.network                 -> network-infra "Анонсирует IP-адреса через BGP"
network-load-balancer       -> dkp.ingress-nginx "Балансирует HTTP/HTTPS запросы"
dkp.ingress-nginx           -> dkp.console
dkp.ingress-nginx           -> dkp.cluster-control-plane
dkp.ingress-nginx           -> dkp.user-authn
dkp.ingress-nginx           -> dkp.dashboard
dkp.ingress-nginx           -> dkp.documentation
dkp.ingress-nginx           -> dkp.deckhouse-tools
dkp.ingress-nginx           -> dkp.upmeter
dkp.ingress-nginx           -> dkp.prometheus
dkp.kubernetes-api-proxy    -> dkp.cluster-control-plane
dkp.metallb                 -> network-infra "Анонсирует IP-адреса через BGP"
dkp.metallb                 -> dkp.cluster-control-plane

// Связи в модуле ingress-nginx
users                           -> network-load-balancer "1. Отправляют HTTP/HTTPS запросы"
network-load-balancer           -> dkp.ing-controller "2. Балансирует запросы на инстансы controller-nginx [80/443 TCP]"
network-load-balancer           -> dkp.ing-istio-proxy "Трафик в сторону Service CIDR перехватывается сайдкаром Istio [80/443 TCP]"
dkp.ing-istio-proxy             -> dkp.ing-controller "Трафик, перехваченный Istio"
dkp.ing-controller              -> dkp.ing-istio-proxy "Трафик, перехваченный Istio"
dkp.ing-istio-proxy             -> dkp.user-app "Трафик, перехваченный Istio"
dkp.ing-controller              -> dkp.console-dex-authenticator "3. Отправляет запросы на аутентификацию [443 TCP]"
// dkp.ing-controller              -> dkp.kubeconfig-generator "Пересылает запросы на генерацию kubeconfig [5555 TCP]" // Deprecated maksim.nabokikh@flant.com
dkp.ing-controller              -> dkp.user-app-dex-authenticator "3. Отправляет запросы на аутентификацию [443 TCP]"
dkp.ing-controller              -> dkp.frontend "6. Пересылает авторизованный запрос пользователя на просмотр Web-консоли"
dkp.ing-controller              -> dkp.user-app "6. Пересылает авторизованный запрос пользователя"
dkp.ing-controller              -> dkp.user-app-with-dex-client "3. Пересылает запрос пользователя"
dkp.ing-controller              -> dkp.kube-apiserver "Синхронизирует конфигурацию NGINX с ресурсами Ingress"
dkp.kube-apiserver              -> dkp.ing-validator "Валидирует Ingress на вебхук svc nginx-admission [443:8443 TCP]"       
// dkp.kube-apiserver              -> dkp.etcd "Сохраняет проверенный ресурс в базе данных"
dkp.ing-kube-rbac-proxy         -> dkp.kube-apiserver "Выполняет авторизацию запросов"
dkp.ing-kube-rbac-proxy         -> dkp.ing-controller "Пересылает авторизованные запросы на метрики и healthcheck [10254 TCP]"
dkp.ing-kube-rbac-proxy         -> dkp.ing-protobuf-exporter "Пересылает авторизованные запросы на статистику Nginx [9091 TCP]"
dkp.ing-controller              -> dkp.ing-protobuf-exporter "Отправляет статистистику Nginx в формате protobuf (TCP)"
dkp.kcm-kruise                  -> dkp.kube-apiserver "Управляет ресурсом Advanced DaemonSet (controller-nginx)"
dkp.kube-apiserver              -> dkp.kcm-kruise "Валидирует объекты API OpenKruize на kruise-webhook-service [443:9876 TCP]"
dkp.kcm-kube-rbac-proxy         -> dkp.kube-apiserver "Выполняет авторизация запросов"
dkp.kcm-kube-rbac-proxy         -> dkp.kcm-kruise-state-metrics "Пересылает авторизованные запросы на метрики и healthcheck [8082 TCP]"
dkp.kcm-kruise-state-metrics    -> dkp.kube-apiserver "Следит за состоянием объектов API OpenKruise"
dkp.prometheus-main             -> dkp.ing-kube-rbac-proxy "Собирает метрики и статистику Nginx [4207 TCP]"
dkp.prometheus-main             -> dkp.kcm-kube-rbac-proxy "Собирает метрики [10354 TCP]"

// Связи в подсистеме Managed Services
dkp.managed-postgres -> dkp.cluster-control-plane

// Связи в подсистеме Observability
dkp.observability-subsystem -> logging-external "Отправляет логи и события безопасности"
dkp.observability-subsystem -> alert-receivers "Отправляет алерты"
dkp.log-shipper         -> logging-external "Отправляет логи и события безопасности"
dkp.prometheus          -> alert-receivers "Отправляет алерты"
dkp.operator-prometheus -> dkp.cluster-control-plane
dkp.prometheus          -> dkp.cluster-control-plane

// Связи в модуле log-shipper
dkp.vector                  -> logging-external "Отправляет логи"
dkp.vector                  -> dkp.loki "Кратковременное хранение логов"
dkp.vector-reloader         -> dkp.kube-apiserver "Перезапускает pod vector при изменении секрета с конфигурацией"
dkp.vector-kube-rbac-proxy  -> dkp.kube-apiserver "Выполняет авторизацию запросов"
dkp.vector-kube-rbac-proxy  -> dkp.vector "Пересылает авторизованные запросы на метрики [9090 TCP]"
dkp.prometheus-main         -> dkp.vector-kube-rbac-proxy "Собирает метрики [10254 TCP]"
dkp.vector                  -> dkp.elasticsearch "Отправляет логи"
dkp.vector                  -> dkp.logstash "Отправляет логи"
dkp.vector                  -> dkp.kafka "Отправляет логи"
dkp.vector                  -> dkp.splunk "Отправляет логи"
dkp.vector                  -> dkp.loki-custom "Отправляет логи"
dkp.vector                  -> dkp.log-files "Читает локальные файлы, доступные на узле"
dkp.vector                  -> dkp.log-user-app "Собирает логи с подов"


// Связи в модуле loki
dkp.loki-kube-rbac-proxy    -> dkp.kube-apiserver "Выполняет авторизацию запросов"
dkp.loki-kube-rbac-proxy    -> dkp.loki "Пересылает авторизованные запросы [3101 TCP]"
dkp.grafana-v10             -> dkp.loki-kube-rbac-proxy "Использует Loki в качестве источника данных [3100 TCP]"
dkp.prometheus-main         -> dkp.loki-kube-rbac-proxy "Собирает метрики [3100 TCP]"

// Связи в модуле prometheus
dkp.console                 -> dkp.grafana-v10 "Отображает панель Grafana в Web-консоли"

// Связи в подсистеме Security
dkp.security                    -> cert-issuers "Выпускает сертификаты"
dkp.admission-policy-engine     -> dkp.cluster-control-plane
dkp.cert-manager                -> cert-issuers "Выпускает сертификаты"
dkp.cert-manager                -> dkp.cluster-control-plane
dkp.operator-trivy              -> dkp.cluster-control-plane
dkp.runtime-audit-engine        -> dkp.cluster-control-plane
dkp.secret-copier               -> dkp.cluster-control-plane
dkp.secrets-store-integration   -> dkp.cluster-control-plane

// Связи в подсистеме Storage
dkp.storage                 -> storage-infra "Управляет томами"
dkp.local-path-provisioner  -> dkp.cluster-control-plane
dkp.snapshot-controller     -> dkp.cluster-control-plane
dkp.csi-modules             -> storage-infra "Управляет томами"

// Связи в подсистеме Virtualiztion
dkp.virtualization           -> dkp.cluster-control-plane
