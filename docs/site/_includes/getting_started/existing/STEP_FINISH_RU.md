<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>


{::options parse_block_html="false" /}

<div markdown="1">
## Все установлено, настроено и работает!

Рассмотрим дальнейшие возможности Deckhouse Kubernetes Platform, открывающиеся сразу после установки.

Для доступа к внутрикластерной документации выделен домен `deckhouse` в соответствии с установленным [шаблоном DNS-имен](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate). Например, для шаблона DNS-имен `%s.1.2.3.4.sslip.io`, веб-интерфейс документации будет доступен по адресу `https://deckhouse.1.2.3.4.sslip.io`.

Доступ к документации ограничен аутентификацией (больше вариантов аутентификации можно получить, включив модуль [`user-auth`](/modules/user-authn/)):

- Логин — `admin`
- Пароль сгенерирован автоматически. Узнать его можно выполнив команду:

  - Для Deckhouse 1.46 и новее:

    ```bash
    kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.internal.auth.password'"
    ```

  - Для Deckhouse 1.45 и старее:

    ```bash
    kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values deckhouse-web -o json | jq -r '.deckhouseWeb.internal.auth.password'"
    ```

  {% offtopic title="Пример вывода..." %}
```
$ kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- sh -c "deckhouse-controller module values documentation -o json | jq -r '.internal.auth.password'" 
3aE7nY1VlfiYCH4GFIqA
```
  {% endofftopic %}
</div>

{% include getting_started/global/partials/FINISH_CARDS_RU.md %}
