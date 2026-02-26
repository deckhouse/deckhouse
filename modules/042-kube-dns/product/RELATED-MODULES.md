# Связанные модули с kube-dns

## Прямые зависимости и интеграции

### 1. **350-node-local-dns** (EE edition)

**Тип связи:** Требует kube-dns, оптимизирует его работу

**Описание:**
- Модуль требует наличия kube-dns (`requirements.modules.kube-dns: '>= 0.0.0'`)
- Разворачивает кэширующий DNS сервер на каждом узле кластера
- При наличии node-local-dns, кэш в kube-dns автоматически отключается (строка 36 в configmap.yaml)
- Использует ClusterIP адрес сервиса kube-dns для перенаправления запросов
- Значительно улучшает производительность DNS за счет локального кэширования на узлах

**Взаимодействие:**
- node-local-dns перехватывает DNS запросы на уровне узла
- Не кэшированные запросы перенаправляются в kube-dns
- Снижает нагрузку на kube-dns за счет локального кэширования

### 2. **340-monitoring-kubernetes**

**Тип связи:** Мониторинг и обнаружение

**Описание:**
- Определяет тип DNS реализации в кластере (CoreDNS или kube-dns)
- Создает метрики и алерты для мониторинга DNS
- Использует обнаружение Deployment kube-dns для настройки мониторинга

**Взаимодействие:**
- Хук `cluster_dns_implementation.go` проверяет наличие kube-dns
- Устанавливает `monitoringKubernetes.internal.clusterDNSImplementation = "coredns"`
- Создает PodMonitor и PrometheusRules для мониторинга DNS

### 3. **200-operator-prometheus**

**Тип связи:** Сбор метрик

**Описание:**
- Создает PodMonitor для сбора метрик CoreDNS
- PodMonitor создается только при наличии модуля operator-prometheus
- Метрики доступны через порт 9153 в подах CoreDNS

**Взаимодействие:**
- PodMonitor автоматически создается в namespace d8-monitoring
- Собирает метрики через kube-rbac-proxy (порт https-metrics)
- Метрики включают: количество запросов, время ответа, ошибки, статистику кэша

### 4. **002-deckhouse**

**Тип связи:** Валидация конфигурации

**Описание:**
- Валидирует, что publicDomainTemplate не совпадает с clusterDomainAliases
- Webhook проверяет конфликты между доменами
- Защищает от неправильной конфигурации доменов

**Взаимодействие:**
- Webhook `public-domain-template` проверяет конфигурацию kube-dns
- Сравнивает publicDomainTemplate с clusterDomain и clusterDomainAliases
- Блокирует создание конфликтующих конфигураций

### 5. **110-istio**

**Тип связи:** Использование clusterDomain

**Описание:**
- Использует clusterDomain для конфигурации Service Mesh
- Настраивает trustDomain, clusterName на основе clusterDomain
- Требует рестарт всех подов Istio при смене clusterDomain

**Взаимодействие:**
- Istio использует clusterDomain для идентификации кластера в мультикластере
- При смене clusterDomain необходимо перезапустить все поды Istio
- ClusterDomainAliases могут использоваться для миграции без простоев

## Косвенные зависимости

### 6. **040-control-plane-manager**

**Тип связи:** Конфигурация кластера

**Описание:**
- Управляет clusterDomain через ClusterConfiguration
- При смене clusterDomain требуется координация с kube-dns
- Настраивает certSANs и serviceAccount для поддержки clusterDomainAliases

**Взаимодействие:**
- При миграции clusterDomain необходимо настроить:
  - `apiserver.certSANs` с новым и старым доменом
  - `apiserver.serviceAccount.additionalAPIAudiences` и `additionalAPIIssuers`
  - `kubeDns.clusterDomainAliases` для поддержки обоих доменов

### 7. **040-node-manager**

**Тип связи:** Распределение подов

**Описание:**
- Управляет узлами с ролью `kube-dns` для размещения подов CoreDNS
- Определяет количество реплик kube-dns на основе ролей узлов
- Логика: kube-dns узлы + master узлы (макс. master + 2)

**Взаимодействие:**
- Поды kube-dns размещаются на узлах с ролью `kube-dns` или `system`
- Если нет специальных узлов, используются master узлы
- Количество реплик зависит от количества узлов с соответствующими ролями

