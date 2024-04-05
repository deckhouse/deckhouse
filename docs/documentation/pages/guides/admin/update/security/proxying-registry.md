---
title: Доступ из изолированных контуров container registry с фиксированным набором IP-адресов
permalink: ru/update/security/proxying-registry/
lang: ru
---

Deckhouse Kubernetes Platform можно настроить на работу с проксирующим registry внутри закрытого контура, для этого выполните следующие шаги: 

1. Установите следующие параметры в ресурсе `InitConfiguration`:

   * `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа Deckhouse EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
   * `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

2. При разрешенном анонимном доступе к образам Deckhouse Kubernetes Platform в стороннем registry, удостоверьтесь, что `registryDockerCfg` выглядит следующим образом:

   ```json
   {"auths": { "<PROXY_REGISTRY>": {}}}
   ```

   > Приведенное значение должно быть закодировано в Base64.

3. Если для доступа к образам Deckhouse Kubernetes Platform в стороннем registry необходима аутентификация, удостоверьтесь, что `registryDockerCfg` выглядит следующим образом:

   ```json
   {"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
   ```

   где:

   * `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
   * `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
   * `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
   * `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

   > Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

3. Чтобы настроить нестандартные конфигурации сторонних registry в ресурсе `InitConfiguration`, используйте еще два параметра:

   * `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
   * `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.
   
      <div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

