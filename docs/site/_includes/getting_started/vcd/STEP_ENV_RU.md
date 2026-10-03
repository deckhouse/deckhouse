{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-vcd/environment.html) модуля `cloud-provider-vcd`.

{% alert level="warning" %}
Работоспособность провайдера подтверждена только для шаблонов виртуальных машин на базе Ubuntu 22.04.
{% endalert %}

Выполните предварительные настройки:

1. Получите тенант с ресурсами из [списка необходимых ресурсов VCD](/modules/cloud-provider-vcd/environment.html#список-необходимых-ресурсов-vcd). В виртуальном дата-центре должен быть Edge Gateway. Внутренняя сеть кластера в схеме размещения `WithNAT`, которая используется на этой странице, создаётся автоматически.
1. Получите пользователя с [необходимыми правами](/modules/cloud-provider-vcd/environment.html#права-пользователя).
1. Подготовьте [шаблон виртуальной машины](/modules/cloud-provider-vcd/environment.html#шаблон-виртуальной-машины).
