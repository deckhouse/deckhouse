![resources](https://docs.google.com/drawings/d/e/2PACX-1vRS95L6rJr_SswWphLYYHN9GZLC3I0jpbKXbjr3935kqJdaeBIxmJyejKCOUdLPaKlY2Fk_zzNaGmE9/pub?w=711&h=499)
<!--- Исходник: https://docs.google.com/drawings/d/1UPzygO3w8wsRNHEna2uoYB-69qvW6zDYB5s1OumUOes/edit --->

В данной схеме размещения вместе с кластером создается bastion-хост, через который будет возможен доступ к узлам кластера.

Виртуальные машины будут выходить в интернет через NAT Gateway с общим и единственным source IP.

> **Важно!** В этой схеме размещения NAT Gateway всегда создается в зоне `a`. Если узлы кластера будут заказаны в других зонах, то при проблемах в зоне `a` они также будут недоступны. Другими словами, при выборе схемы размещения `WithNat` доступность всего кластера будет зависеть от работоспособности зоны `a`.
