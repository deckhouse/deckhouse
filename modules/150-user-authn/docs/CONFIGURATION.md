---
title: "Модуль user-authn: конфигурация"
---

Модуль по умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  userAuthnEnabled: "true"
```

## Параметры

* `publishAPI` — настройки публикации API-сервера, чeрез ingress:
  * `enable` — если выставить данный параметр в `true`, то в кластере будет создан ingress в namespace d8-user-authn, который выставляет Kubernetes API наружу.
    * По-умолчанию: `false`.
  * `ingressClass` — ingress-класс, который будет использован для публикации API Kubernetes через Ingress.
  * `whitelistSourceRanges` — массив CIDR, которым разрешено подключение к API.
    * Если параметр не указан, подключение к API не ограничивается по IP.
  * `https` — режим работы HTTPS для Ingress API-сервера:
    * `mode` — режим выдачи сертификатов для данного Ingress-ресурса. Возможные значения `SelfSigned` и `Global`. В случае использования режима `SelfSigned` для ingress ресурса будет выпущен самоподписанный сертификат. В случае использования `Global` будут применены политики из глобальной настройки `global.modules.https.mode`. Т.е. если в глобальной настройке стоит режим `CertManager` с clusterissuer `letsencrypt`, то для ingress ресурса будет заказан сертификат Lets Encrypt.
      * По-умолчанию: `SelfSigned`
    * `global` — дополнительный параметр для режима `Global`;
      * `kubeconfigGeneratorMasterCA` — если у вас перед Ingress есть внешний балансер, который терминирует HTTPS трафик, то тут необходимо вставить CA от сертификата на балансировщике, что бы kubectl мог достучаться до API-сервера;
         * В качестве CA можно указать сам сертификат с внешнего балансера, если по какой-то причине вы не можете получить подписавший его CA. Но нужно помнить, что после обновления сертификата на балансере сгенерированные ранее kubeconfig'и перестанут работать.
* `kubeconfigGenerator` — массив, в котором указываются дополнительные возможные способы доступа к API. Это может быть полезно, в случае если вы не хотите предоставить доступ к API-кластера через Ingress, а хотите предоставить доступ другими способами (например, с бастион-хоста или через OpenVPN).
  * `id` — имя способа доступа к API-серверу (без пробелов, маленькими буквами);
  * `masterURI` — адрес API-сервера;
    * Если вы планируете использовать TCP прокси, то для адреса TCP-прокси должен быть сконфигурирован сертификат на стороне API-сервера. Например, в случае, если у вас API-сервера'а слушают на трех разных адресах (`192.168.0.10`, `192.168.0.11` и `192.168.0.12`), а ходить к API-серверу клиент будет, через TCP-балансер (пусть будет `192.168.0.15`), то вам необходимо перегенерировать сертификаты для API-серверов:
      * отредактировать `kubeadm-config`: `kubectl -n kube-system edit configmap kubeadm-config` добавив в `.apiServer.certSANs` адрес `192.168.0.15`;
      * сохранить получившийся конфиг: `kubeadm config view > kubeadmconf.yaml`;
      * удалить старые сертификаты API-сервера: `mv /etc/kubernetes/pki/apiserver.* /tmp/`;
      * перевыпустить новые сертификаты: `kubeadm init phase certs apiserver --config=kubeadmconf.yaml`;
      * перезапустить контейнер с API-сервером: `docker ps -a | grep 'kube-apiserver' | grep -v pause| awk '{print $1}' | xargs docker restart`;
      * повторить данное действие для всех мастеров.
  * `masterCA` — CA для доступа к API.
    * Если данный параметр не указать, то будет автоматически использован Kubernetes CA.
    * При публикации через HTTP-прокси, который терминирует HTTPS трафик, рекомендуется использовать самоподписанный сертификат, который и указать в настоящем параметре.
  * `description` — описание способа доступа к API-серверу, которое показывается пользователю (в списке).
* `idTokenTTL` — данный параметр отвечает за время жизни id токена (указывается с суффиксом s, m или h);
  * По-умолчанию — 10 минут.
  * Пример: `1h`
* `highAvailability` — ручное управление режимом отказоустойчивости.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.deckhouse.io/vsphere-csi-driver":""}` или `{"node-role.deckhouse.io/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет настроено значение `[{"key":"dedicated.deckhouse.io","operator":"Equal","value":"vsphere-csi-driver"},{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `ingressClass` — класс Ingress-контроллера, который используется для dex и kubeconfig-generator.
  * Опциональный параметр, по умолчанию используется глобальное значение `modules.ingressClass`.
* `https` — выбираем, какой тип сертификата использовать для dex и kubeconfig-generator.
  * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
  * `mode` — режим работы HTTPS:
    * `Disabled` — при данном значении модуль автоматически отключается.
    * `CertManager` — dex и kubeconfig-generator будут работать по HTTPS и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
    * `CustomCertificate` — dex и kubeconfig-generator будут работать по HTTPS используя сертификат из namespace `d8-system`;
    * `OnlyInURI` — dex и kubeconfig-generator будут работать по HTTP (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с HTTPS-схемой.
  * `certManager`
    * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для dex и kubeconfig-generator (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
  * `customCertificate`
    * `secretName` — указываем имя secret'а в namespace `d8-system`, который будет использоваться для dex и kubeconfig-generator (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
* `controlPlaneConfigurator` — настройки параметров для модуля автоматической настройки kube-apiserver [control-plane-configurator]({{ site.baseurl }}/modules/160-control-plane-configurator/).
  * `enabled` — использовать ли control-plane-configurator для настройки OIDC в kube-apiserver.
    * По-умолчанию `true`.
  * `dexCAMode` — как вычислить CA, который будет использован при настройке kube-apiserver.
    * Значения:
      * `FromIngressSecret` — извлечь CA или сам сертификат из секрета, который используется в ингрессе. Если вы используете самоподписанные сертификаты на ингрессах — это ваш вариант.
      * `Custom` — использовать CA указанный явно, в параметре `dexCustomCA` (см. ниже). Этот вариант уместен, например, если вы используете внешний https-балансер перед ингрессами и на этом балансировщике используется самоподписанный сертификат.
      * `DoNotNeed` — CA не требуется (например, при использовании публичного LE или других TLS-провайдеров).
    * По-умолчанию — `DoNotNeed`.
  * `dexCustomCA` — CA, которая будет использована в случае `dexCAMode` = `Custom`.
    * Формат — обычный текст, без base64.
    * Необязательный параметр.

### Пример конфигурации
{% raw %}

```yaml
  userAuthnEnabled: "true"
  userAuthn: |
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enable: true
```
{% endraw %}

Автоматический деплой [oauth2-proxy](https://github.com/pusher/oauth2_proxy) в namespace вашего приложения и подключения его к dex происходит при создании Custom Resource [`DexAuthenticator`](cr.html#dexauthenticator).

**Важно!** Так как использование OpenID Connect по протоколу HTTP является слишком значительной угрозой безопасности (что подтверждается, например, тем что kubernetes api-сервер не поддерживает работу с OIDC по HTTP), данный модуль можно установить только при включенном HTTPS (`https.mode` выставить в отличное от `Disabled` значение или на уровне кластера, или в самом модуле).

**Важно!** При включении данного модуля аутентификация во всех веб-интерфейсах перестанет использовать HTTP Basic Auth и переключится на dex (который, в свою очередь, будет использовать настроенные вами внешние провайдеры).
Для настройки kubectl необходимо перейти по адресу: `https://kubeconfig.<modules.publicDomainTemplate>/`, авторизоваться в настроенном внешнем провайдере и скопировать shell команды к себе в консоль.

**Важно!** Для работы аутентификации в dashboard и kubectl требуется [донастройка API-сервера](usage.html#настройка-kube-apiserver). Для автоматизации этого процесса реализован модуль [control-plane-configurator](/modules/160-control-plane-configurator/), который включён по умолчанию.

