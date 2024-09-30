![resources](https://docs.google.com/drawings/d/e/2PACX-1vSTIcQnxcwHsgANqHE5Ry_ZcetYX2lTFdDjd3Kip5cteSbUxwRjR3NigwQzyTMDGX10_Avr_mizOB5o/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

Создаётся внутренняя сеть кластера со шлюзом в публичную сеть, узлы не имеют публичных IP-адресов. Для master-узла заказывается floating IP.

{% alert level="warning" %}
Если провайдер не поддерживает SecurityGroups, то все приложения, запущенные на узлах с floating IP, будут доступны по публичному IP.
Например, kube-apiserver на мастер-узлах будет доступен по порту 6443. Чтобы этого избежать, рекомендуется использовать схему размещения `SimpleWithInternalNetwork`.
{% endalert %}
