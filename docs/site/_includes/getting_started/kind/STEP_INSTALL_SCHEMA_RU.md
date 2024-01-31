[kind](https://kind.sigs.k8s.io/) — утилита для запуска локальных кластеров Kubernetes, которая в качестве узлов кластера использует контейнеры. Создана преимущественно для тестирования самого Kubernetes, но может использоваться для локальной разработки или CI.

Установка Deckhouse на kind, рассматриваемая далее, позволит вам за менее чем 15 минут получить локальный кластер Kubernetes с установленным Deckhouse. Такой вариант развертывания Deckhouse позволит вам быстро развернуть Deckhouse и познакомиться с основными его возможностями.

Deckhouse будет установлен в **минимальной** конфигурации с включенным [мониторингом](/documentation/v1/modules/300-prometheus/) на базе Grafana. Некоторые функции, такие как [управление узлами](/documentation/v1/modules/040-node-manager/) и [управление control-plane](/documentation/v1/modules/040-control-plane-manager/), работать не будут. Для упрощения при работе с DNS используется сервис [sslip.io](https://sslip.io).

{% alert level="warning" %}
Некоторые провайдеры блокируют работу sslip.io и подобных сервисов. Если вы столкнулись с такой проблемой, пропишите нужные домены в `hosts`-файл локально, или направьте реальный домен и исправьте [шаблон DNS-имен](../../documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate).
{% endalert %}

{% comment %}
При использовании kind на Windows мониторинг (Grafana, Prometheus) может быть недоступен или работать некорректно. Это связано с необходимостью использовать специальное ядро для WSL. Решение проблемы описано [в документации kind](https://kind.sigs.k8s.io/docs/user/using-wsl2/#kubernetes-service-with-session-affinity).
{% endcomment %}

{% offtopic title="Минимальные требования к компьютеру..." %}
- Операционная система macOS или Linux (работа на Windows не поддерживается).
- Установленный container runtime (docker, containerd) и docker-клиент.
    - Для работы контейнеров должно быть выделено не менее 4 CPU и 8 Гб оперативной памяти (_Settings -> Resources_ в Docker Desktop)
    - В macOS должен быть включен параметр `Enable privileged port mapping` (_Settings -> Advanced -> Enable privileged port mapping_ в Docker Desktop).
- HTTPS-доступ до хранилища образов контейнеров `registry.deckhouse.ru`.
{% endofftopic %}

## Установка

Развертывание кластера Kubernetes и установка в него Deckhouse выполняются с помощью [Shell-скрипта](https://github.com/deckhouse/deckhouse/blob/main/tools/kind-d8.sh):
- Выполните следующую команду для установки Deckhouse **Community Edition**:
  {% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)"
```
  {% endsnippetcut %}
- Либо выполните следующую команду для установки Deckhouse **Enterprise Edition**, указав лицензионный ключ:
  {% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)" -- --key <LICENSE_KEY>
```
  {% endsnippetcut %}

По окончании установки инсталлятор выведет пароль пользователя `admin` для доступа в Grafana, которая будет доступна по адресу [http://grafana.127.0.0.1.sslip.io](http://grafana.127.0.0.1.sslip.io).

{% offtopic title="Пример вывода..." %}
```text
Waiting for the Ingress controller to be ready.........................................
Ingress controller is running.

You have installed Deckhouse Kubernetes Platform in kind!

Don't forget that the default kubectl context has been changed to 'kind-d8'.

Run 'kubectl --context kind-d8 cluster-info' to see cluster info.
Run 'kind delete cluster --name d8' to remove cluster.

Provide following credentials to access Grafana at http://grafana.127.0.0.1.sslip.io/ :

    Username: admin
    Password: LlF7X67BvgRO74LNWXHi

The information above is saved to /home/user/.kind-d8/info.txt file.

Good luck!
```
{% endofftopic %}

Пароль пользователя `admin` для Grafana также можно узнать выполнив команду:
{% snippetcut selector="kind-get-password" %}
```shell
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- sh -c "deckhouse-controller module values prometheus -o json | jq -r '.prometheus.internal.auth.password'"
```
{% endsnippetcut %}
