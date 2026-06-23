![resources](/images/gs/cloud-provider-yandex/layout-standard.png)
<!--- Исходник: https://docs.google.com/drawings/d/1WI8tu-QZYcz3DvYBNlZG4s5OKQ9JKyna7ESHjnjuCVQ/edit --->

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов, а будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud.
{% endalert %}

{% alert level="warning" %}
Так как в данной схеме размещения узлы создаются без публичных IP-адресов, для установки кластера у master-узла должен быть SSH-доступ с машины, на которой запускается `dhctl`: напрямую по приватной сети или через bastion-хост.
Если master-узел недоступен напрямую, запускайте установку с параметрами `--ssh-bastion-host`, `--ssh-bastion-user` и, при необходимости, `--ssh-bastion-port`.
{% endalert %}
