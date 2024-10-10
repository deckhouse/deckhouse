# Checklist

1. [ ] Написаны/выбраны провайдеры Terraform, CCM, CSI, CAPI
2. [ ] Deckhouse cloud-provider описан в candi
3. [ ] В репозиторий https://fox.flant.com/deckhouse скопированы исходники провайдера Terraform и сделана сборка на werf в качестве образа в модуле terraform-manager
4. [ ] Заведен модуль 030-cloud-provider-***** с openapi и хуками, применяющими CRD InstanceClass/Cluster-API-Provider и собирающими cloud provider discovery data
5. [ ] В candi подготовлены нужные terraform layout (как минимум standard) с базовой инфраструктурой и ресурсами для поднятия первого master-узла
6. [ ] Написан cloud-data-discoverer
7. [ ] В репозиторий https://fox.flant.com/deckhouse скопированы исходники провайдеров Cluster API Provider, CCM, CSI
8. [ ] Сделана сборка CCM на werf в качестве образов модуля cloud-provider
9. [ ] Сделана сборка CSI на werf в качестве образов модуля cloud-provider
10. [ ] Сделана сборка Cluster API Provider на werf в качестве образов модуля cloud-provider
11. [ ] Подготовлены ресурсы для сущностей Cluster API - шаблоны Cluster и MachineTemplate с вычислением его контрольной суммы
12. [ ] Подготовлены шаблоны манифестов для выката в кластер CCM, CSI, Cluster-API-Provider включая RBAC и секреты с доступами
13. [ ] Подготовлены шаблоны registration.yaml, namespace.yaml
14. [ ] Запущен локально make generate в корневой директории с исходным кодом Deckhouse
