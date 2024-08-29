<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Рассмотрим дальнейшие возможности Deckhouse, открывающиеся сразу после установки.

Для доступа к внутрикластерной документации выделен домен `deckhouse` в соответствии с установленным [шаблоном DNS-имен](../../products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate). Например, для шаблона DNS-имен `%s.1.2.3.4.sslip.io`, веб-интерфейс документации будет доступен по адресу `https://deckhouse.1.2.3.4.sslip.io`.

Доступ к документации ограничен basic-аутентификацией (больше вариантов аутентификации можно получить включив модуль [user-auth](../../products/kubernetes-platform/documentation/v1/modules/150-user-authn/)):  
- Логин — `admin`
- Пароль сгенерирован автоматически. Узнать его можно выполнив команду:

  - Для Deckhouse 1.46 и новее:

    {% snippetcut %}
```bash
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.internal.auth.password'"
```
{% endsnippetcut %}

  - Для Deckhouse 1.45 и старее:
    
    {% snippetcut %}
```bash
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values deckhouse-web -o json | jq -r '.deckhouseWeb.internal.auth.password'"
```
{% endsnippetcut %}

  {% offtopic title="Пример вывода..." %}
```
$ kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.internal.auth.password'" 
3aE7nY1VlfiYCH4GFIqA
```
  {% endofftopic %}
</div>

{% include getting_started/global/partials/FINISH_CARDS_RU.md %}
