---
title: Что делать, если при включенном VPN контейнер с установщиком не может получить доступ к сети компьютера, с которого выполняется bootstrap кластера?
lang: ru
---

Если на компьютере, с которого выполняется bootstrap кластера, установлен VPN, может возникнуть проблема с доступом контейнера с установщиком (или графического инсталлятора) к сетевому стеку компьютера. Из-за этого, например, графический инсталлятор может не отображаться в браузере.

Решить проблему можно одним из следующих способов:

- Отключить VPN на компьютере и перезапустить контейнер с установщиком.
- Если нет возможности отключить VPN (например, bootstrap кластера выполняется в сети VPN), используйте при запуске контейнера с установщиком параметр `--network host` (**для Docker Desktop на Mac OS параметр доступен, [начиная с версии 4.34.0](https://docs.docker.com/desktop/release-notes/#4340)**). Это позволит контейнеру получить доступ к сетевому стеку хоста.

  Пример запуска контейнера с установщиком на компьютере с включенным VPN, с использованием параметра `--network host`:

  ```shell
  docker run --network host --pull=always -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" -v "$PWD/dhctl-tmp:/tmp/dhctl" registry.deckhouse.ru/deckhouse/ce/install:early-access bash
  ```

   Пример запуска графического инсталлятора на компьютере с включенным VPN, с использованием параметра `--network host`:

  ```shell
  docker run --network host --rm --pull always -v $HOME/.d8installer:$HOME/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r $HOME/.d8installer
  ```
