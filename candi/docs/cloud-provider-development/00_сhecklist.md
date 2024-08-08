# Checklist

1. [ ] Провайдер описан в candi, выбраны провайдеры Terraform, Cluster API, CSI, CCM, подготовлен InstanceClass;
2. [ ] В FOX склонированы исходники провайдера Terraform и сделана сборка на werf в качестве образа в модуле terraform-manager;
3. [ ] Заведен модуль 030-cloud-provider-***** с openapi и хуками, применяюшими CRD InstanceClass/Cluster-API-Provider и собирающими cloud provider discovery data;
4. [ ] В candi подготовлены нужные terraform layout (как минимум standard) с базовой инфраструктурой и ресурсами для поднятия первого master-узла;
5. [ ] Написан cloud-data-discoverer;
6. [ ] В FOX склонированы исходники провайдеров Cluster API Provider, CSI, CCM;
7. [ ] Сделана сборка CCM на werf в качестве образов модуля cloud-provider;
8. [ ] Сделана сборка Cluster API Provider на werf в качестве образов модуля cloud-provider;
9. [ ] Сделана сборка CSI на werf в качестве образов модуля cloud-provider;
10. [ ] Подготовлены ресурсы для сущностей Cluster API - шаблоны Cluster и MachineTemplate с рассчетом его контрольной суммы;
11. [ ] Подготовлены шаблоны манифестов для выката в кластер CCM, CSI, Cluster-API-Provider включая RBAC и секреты с доступами;
12. [ ] Подготовлены шабоны cni.yaml, registration.yaml, namespace.yaml
