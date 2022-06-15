![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

**Внимание!**

В данной схеме размещения не происходит управление `SecurityGroups`, а подразумевается что они были ранее созданы.
Для настройки политик безопасности необходимо явно указывать `additionalSecurityGroups` в OpenStackClusterConfiguration
для masterNodeGroup и других nodeGroups, и `additionalSecurityGroups` при создании `OpenStackInstanceClass` в кластере.
