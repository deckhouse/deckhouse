---
title: Что делать, если при включенном VPN контейнер с установщиком не может получить доступ к сети компьютера, с которого выполняется bootstrap кластера?
lang: ru
---

Если на компьютере, с которого выполняется bootstrap кластера, включен VPN, контейнер с установщиком (включая графический инсталлятор) может потерять доступ к сети хоста. Из-за этого, например, веб-интерфейс графического инсталлятора может не открываться в браузере.

Решить проблему можно одним из следующих способов:

- Отключите VPN на компьютере и перезапустите контейнер с установщиком.
- Если отключить VPN нельзя (например, bootstrap кластера выполняется в сети VPN), используйте при запуске контейнера с установщиком параметр `--network host` (**для Docker Desktop на Mac OS параметр доступен, [начиная с версии 4.34.0](https://docs.docker.com/desktop/release-notes/#4340)**). Это позволит контейнеру получить доступ к сети хоста.

  Пример запуска контейнера с установщиком при включённом VPN, с использованием параметра `--network host`:

  ```shell
  docker run --network host --pull=always -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" -v "$PWD/dhctl-tmp:/tmp/dhctl" registry.deckhouse.ru/deckhouse/ce/install:early-access bash
  ```

   Пример запуска графического инсталлятора при включённом VPN, с использованием параметра `--network host`:

  ```shell
  docker run --network host --rm --pull always -v $HOME/.d8installer:$HOME/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r $HOME/.d8installer
  ```
