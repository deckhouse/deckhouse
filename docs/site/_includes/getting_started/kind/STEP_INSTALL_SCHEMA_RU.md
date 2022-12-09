[kind](https://kind.sigs.k8s.io/) — утилита для запуска локальных кластеров Kubernetes, которая в качестве узлов кластера использует контейнеры. Создана преимущественно для тестирования самого Kubernetes, но может использоваться для локальной разработки или CI.

Установка Deckhouse на kind, рассматриваемая далее, позволит вам за менее чем 10 минут получить кластер Kubernetes с установленным Deckhouse.

Такой вариант развертывания Deckhouse позволит вам быстро развернуть Deckhouse и познакомиться с основными его возможностями.

Обратите внимание, что некоторые функции, такие как [управление узлами](/documentation/v1/modules/040-node-manager/) и [управление control-plane](/documentation/v1/modules/040-control-plane-manager/), работать не будут.

Данное руководство предлагает установку Deckhouse в **минимальной** конфигурации с включенным [мониторингом](/documentation/v1/modules/300-prometheus/) на базе Grafana. Для упрощения при работе с DNS используется сервис [nip.io](https://nip.io). 

## Процесс установки

Для установки потребуется персональный компьютер, отвечающий следующим требованиям:
- Операционная система macOS, Windows или Linux.
- Не менее чем 4Гб оперативной памяти.
- Установленный container runtime (docker, containerd) и docker-клиент.
- HTTPS-доступ до хранилища образов контейнеров `registry.deckhouse.io`.

На этом компьютере будет развернут кластер Kubernetes, в который будет установлен Deckhouse. 

Вы можете выбрать следующие варианты установки:
<ul>
<li>Самостоятельно пройти этапы руководства.</li>
<li>Воспользоваться <a href="https://github.com/deckhouse/deckhouse/blob/main/tools/kind-d8.sh">скриптом установки</a> для Debian-подобных Linux-дистрибутивов или macOS:
  <ul>
  <li>Выполните следующую команду для установки Deckhouse Community Edition:<br/>
{% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)"
```
{% endsnippetcut %}
  </li>
  <li>Либо выполните следующую команду для установки Deckhouse Enterprise Edition, указав лицензионный ключ:<br/>
{% snippetcut selector="kind-install" %}
```shell
bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/kind-d8.sh)" -- --key <LICENSE_KEY>
```
{% endsnippetcut %}
  </li>
  <li>Перейдите к <a href="step5.html">финальному шагу</a> руководства.</li>
  </ul>
</li>
</ul>

По окончании установки вы сможете самостоятельно включить интересующие вас модули. Воспользуйтесь [документацией](/documentation/), чтобы получить об этом необходимую информацию. При возникновении вопросов вы можете попросить помощи [сообщества](/community/about.html).
