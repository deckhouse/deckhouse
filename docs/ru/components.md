# Компоненты Antiopa

* устанавливает kube-lego
* устанавливает ingress controller (необходима соответствующая [настройка](rfc-ingress.md))
* создает роль cluster-admin
* устанавливает отдельный экземпляр tiller
* управляет дополнениями для работы кластера
* подтюнивает кластер

## Что устанавливается?

* Namespace antiopa (можно поменять параметром --namespace в установочном скрипте)
* Deployment antiopa — сама прога, которая запускает модули.
* Secret registrysecret — доступ в docker регистри по указанному gitlab-токену.
* ConfigMap antiopa — конфигурация antiopa для данного конкретного кластера. Меняется по необходимости.
* При первом запуске antiopa инициализирует собственный отдельный instance tiller в том же namespace (antiopa).
