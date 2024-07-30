{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}
{% alert level="warning" %}
Работоспособность провайдера подтверждена только для шаблонов виртуальных машин на базе Ubuntu 22.04.
{% endalert %}

Для начала работы с провайдером необходим созданный тенант с ресурсами, указанными в [документации](/documentation/v1/modules/030-cloud-provider-vcd/environment.html#список-необходимых-ресурсов-vcd).

После получения тенанта, необходимо настроить внутреннюю сеть, EDGE Gateway и подготовить шаблон виртуальной машины. Следуйте инструкциям по настройке окружения в [документации](/documentation/v1/modules/030-cloud-provider-vcd/environment.html) провайдера.
