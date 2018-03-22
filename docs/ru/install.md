# Установка Antiopa

Смотри [скрипт установки](/get-antiopa.sh)




## Переключение уже установленной antiopa на другую ветку

1. `kubectl -n antiopa test edit deploy/antiopa`
2. Меняем образ `registry.flant.com/sys/antiopa:stable`, например, на `registry.flant.com/sys/antiopa:ea`, чтобы переключить на ветку ea.

## Отключение модулей
Модули, которые указаны в ключе *disable-modules* в *ConfigMap* antiopa, будут выключены. Перечисляются через запятую, можно использовать glob'ы.
```
apiVersion: v1
kind: ConfigMap
metadata:
name: antiopa
data:
  disable-modules: test*, kube-dashboard
```

### Пользователь fox.flant.com

Единый пользователь fox.flant.com: antiopa.
Токен: в канале #tech-kubernetes.

Если надо поменять token получаем админа в fox, находим пользователя antiopa, меняем. После смены можно заново вызвать скрипт get-antiopa.sh одним из указанных ниже способов уже с новым token.
