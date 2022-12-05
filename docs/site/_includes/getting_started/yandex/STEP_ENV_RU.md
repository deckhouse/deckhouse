{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для управления ресурсами в Яндекс.Облаке, необходимо создать сервисный аккаунт с правами на редактирование. Подробная инструкция по созданию сервисного аккаунта в Яндекс.Облако доступна в [документации](/documentation/v1/modules/030-cloud-provider-yandex/environment.html). Ниже краткая версия:

Создайте пользователя с именем `deckhouse`. В ответ вернутся параметры пользователя:
{% snippetcut %}
```yaml
yc iam service-account create --name deckhouse
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```
{% endsnippetcut %}

Назначьте роль `editor` вновь созданному пользователю для своего облака:
{% snippetcut %}
```yaml
yc resource-manager folder add-access-binding <folderID> --role editor --subject serviceAccount:<userID>
```
{% endsnippetcut %}

Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
{% snippetcut %}
```yaml
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
{% endsnippetcut %}

<div id="standard-layout-notes" style="display:none" markdown="1">
**Внимание!**

При использовании схемы расположения ресурсов **Standard**, в течение 3х минут после создания базовых сетевых ресурсов для всех подсетей необходимо включить `Cloud NAT`. Если этого не сделать, то процесс bootstrap'а **не сможет завершиться успешно**.

Включить Cloud NAT можно вручную через веб-интерфейс.

Пример:

![Включение NAT](/documentation/v1/images/030-cloud-provider-yandex/enable_cloud_nat_ru.png)
</div>

<script>
$(document).ready(function() {
    if (sessionStorage.getItem('dhctl-layout').toLowerCase() === 'standard') {
        $('#standard-layout-notes').css('display', 'block');
    }
})
</script>
