---
title: "Подпись SSH-сертификатов"
permalink: ru/stronghold/documentation/user/secrets-engines/ssh.html
lang: ru
---

SSH-сертификаты с подписью - самый простой и мощный способ доступа на сервер с точки зрения простоты настройки и независимости от платформы.
Используя возможности Stronghold по созданию CA и функциональность, встроенную в OpenSSH, клиенты могут подключаться к удаленным хостам по SSH, используя свои собственные локальные SSH-ключи.

В этом разделе термин «**клиент**» (client) относится к человеку или машине, выполняющей операцию SSH. Термин «**хост**» (host) относится к удаленной машине.
Если вас это смущает, замените «клиент» на «пользователь».

На этой странице представлен быстрый старт по использованию этого механизма секретов.

## Подпись ключей клиентов

Первоначально необходимо настроить механизм SSH-секретов Stronghold, после чего клиенту будет доступна возможность подписи своего SSH-ключа.
Обычно эти действия выполняет администратор stronghold или команда безопасности.
Также можно автоматизировать эти действия с помощью инструментов управления конфигурацией, таких как Chef, Puppet, Ansible или Salt.

### Создание ключа подписи и настройка роли

Следующие шаги выполняются администратором Stronghold, командой безопасности или средствами управления конфигурацией.

- Смонтируйте механизм секретов. Без этой операции, механизм секретов SSH работать не будет.

```text
$ stronghold secrets enable -path=ssh-client-signer ssh

Successfully mounted 'ssh' at 'ssh-client-signer'!
```

  Эта команда включает механизм секретов SSH по пути `«ssh-client-signer»`.

  Можно подключать один и тот же механизм секретов несколько раз, используя разные аргументы `-path`.

  Имя `«ssh-client-signer»` не является специальным - оно может быть любым, в данной документации будет использоваться `«ssh-client-signer»` в качестве примера.

- Настройте Stronghold c CA для подписи клиентских ключей с помощью метода (endpoint) `/config/ca`.
Если у вас нет внутреннего CA, Stronghold может сгенерировать публичный и приватный ключи для вас.

```text
$ stronghold write ssh-client-signer/config/ca generate_signing_key=true

Key             Value
---             -----
public_key      ssh-rsa AAAAB3NzaC1yc2EA...
```

Если у вас уже есть пара ssh-ключей, укажите части открытого и закрытого ключей в составе команды:

```text
$ stronghold write ssh-client-signer/config/ca \
  private_key="..." \
  public_key="..."
```

Механизм секретов SSH позволяет настраивать несколько сертификатов доверенного центра сертификации (CA) в одном монтировании.
Эта возможность предназначена для облегчения ротации CA. При настройке CA один эмитент (issuer) назначается по умолчанию - его операции будут использоваться во всех случаях, когда при создании роли не указан конкретный эмитент.
Эмитента по умолчанию можно изменить в любой момент, сгенерировав новый CA или обновив его через метод конфигурации, такой подход обеспечивает беспрепятственную ротацию CA.
Независимо от того, сгенерирован или загружен открытый ключ, он доступен через API в методе `/public_key` или через CLI (см. следующий шаг).

- Добавьте открытый ключ во все конфигурации SSH хоста. Этот процесс можно выполнить вручную или автоматизировать с помощью инструмента управления конфигурацией.
Открытый ключ доступен через API и не требует аутентификации.

```text
curl -o /etc/ssh/trusted-user-ca-keys.pem http://127.0.0.1:8200/v1/ssh-client-signer/public_key
```

```text
stronghold read -field=public_key ssh-client-signer/config/ca > /etc/ssh/trusted-user-ca-keys.pem
```

 Добавьте путь, где хранится содержимое открытого ключа, в конфигурационный файл SSH в качестве опции `TrustedUserCAKeys`.

```text
# /etc/ssh/sshd_config
# ...
TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem
```

Перезагрузите службу SSH, чтобы применить изменения.

- Создайте в Stronghold роль с произвольным именем для подписи клиентских ключей.
Из-за реализации некоторых функции сертификата SSH, опции передаются в виде ключ-значение.

Следующий пример добавляет расширение `permit-pty` к сертификату и позволяет пользователю указать собственные значения для `permit-pty` и `permit-port-forwarding` при запросе сертификата.

```text
$ stronghold write ssh-client-signer/roles/my-role -<<"EOH"
{
    "algorithm_signer": "rsa-sha2-256",
    "allow_user_certificates": true,
    "allowed_users": "*",
    "allowed_extensions": "permit-pty,permit-port-forwarding",
    "default_extensions": {
        "permit-pty": ""
    },
    "key_type": "ca",
    "default_user": "ubuntu",
    "ttl": "30m0s"
}
EOH
```

### Аутентификация клиента по SSH

Следующие шаги выполняются клиентом (пользователем), который хочет аутентифицироваться на машинах, управляемых Stronghold.
Эти команды обычно выполняются с локальной рабочей станции клиента.

- Найдите или сгенерируйте открытый ключ SSH. Обычно он расположен по пути `~/.ssh/id_rsa.pub`.

Если у вас нет пары ключей SSH, сгенерируйте их:

```text
ssh-keygen -t rsa -C "user@example.com"
```

- Попросите Stronghold подписать ваш **публичный ключ** (public key). Этот файл обычно заканчивается на `.pub`, а его содержимое начинается с `ssh-rsa ...`.

```text
$ stronghold write ssh-client-signer/sign/my-role \
  public_key=@$HOME/.ssh/id_rsa.pub


 Key             Value
 ---             -----
 serial_number   c73f26d2340276aa
 signed_key      ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1...
```

Результат будет содержать серийный номер (уникальный идентификатор сертификата) и подписанный ключ. Этот подписанный ключ является еще одним открытым ключом.
Чтобы настроить параметры подписи, используйте запрос в формате JSON:

```text
$ stronghold write ssh-client-signer/sign/my-role -<<"EOH"
 {
   "public_key": "ssh-rsa AAA...",
   "valid_principals": "my-user",
   "key_id": "custom-prefix",
   "extensions": {
     "permit-pty": "",
     "permit-port-forwarding": ""
   }
 }
 EOH
```

- Сохраните полученный подписанный открытый ключ на диске. При необходимости ограничьте права доступа.

```text
$ stronghold write -field=signed_key ssh-client-signer/sign/my-role \
  public_key=@$HOME/.ssh/id_rsa.pub > signed-cert.pub
```

Если вы сохраняете сертификат непосредственно рядом с парой ключей SSH, добавьте в имя файла суффикс `-cert.pub` (`~/.ssh/id_rsa-cert.pub`).
При такой схеме именования OpenSSH будет автоматически использовать его при аутентификации.

- (Необязательно) Просмотр включенных расширений, списка пользователей, хостов и метаданных подписанного ключа.

```text
ssh-keygen -Lf ~/.ssh/signed-cert.pub
```

-Выполните на локальной машине команду `ssh`, используя подписанный ключ. Вы должны передать как подписанный открытый ключ, так и соответствующий закрытый ключ в качестве аутентификации для установки SSH-соединения.

```text
ssh -i signed-cert.pub -i ~/.ssh/id_rsa username@10.0.23.5
```

## Подпись ключа хоста (host)

Для дополнительного уровня безопасности мы рекомендуем включить подпись ключей хоста.
Эта функциональность используется вместе с подписью клиентских ключей для обеспечения дополнительного уровня целостности.
Если эта функция включена, агент SSH будет проверять, что удаленный хост является действительным и доверенным, прежде чем попытаться выполнить SSH.
Это снизит вероятность того, что пользователь случайно подключится по SSH к вредоносной машине.

### Настройка подписи ключа

- Подключите Stronghold к другому пути, отличному от пути подписи клиента.

```text
$ stronghold secrets enable -path=ssh-host-signer ssh

Successfully mounted 'ssh' at 'ssh-host-signer'!
```

- Настройте Stronghold c CA для подписания ключей хоста с помощью метода `/config/ca`.
   Если у вас нет внутреннего CA, Stronghold может сгенерировать ключевую пару для вас.

```text
$ stronghold write ssh-host-signer/config/ca generate_signing_key=true

Key             Value
---             -----
public_key      ssh-rsa AAAAB3NzaC1yc2EA...
```

   Если у вас уже есть пара ключей SSH, укажите части открытого и закрытого ключей в составе запроса:

```text
$ stronghold write ssh-host-signer/config/ca \
  private_key="..." \
  public_key="..."
```

Открытый ключ подписывающего хоста доступен через API в методе `/public_key`.

- Увеличение времени TTL сертификата ключа хоста.

```text
stronghold secrets tune -max-lease-ttl=87600h ssh-host-signer
```

- Создайте роль для подписи ключей хоста. Обязательно заполните список разрешенных доменов, установите `allow_bare_domains` или и то, и другое.

```text
$ stronghold write ssh-host-signer/roles/hostrole \
        key_type=ca \
        algorithm_signer=rsa-sha2-256 \
        ttl=87600h \
        allow_host_certificates=true \
        allowed_domains="localdomain,example.com" \
        allow_subdomains=true
```

- Подпишите открытый ключ SSH хоста.

```text
$ stronghold write ssh-host-signer/sign/hostrole \
  cert_type=host \
  public_key=@/etc/ssh/ssh_host_rsa_key.pub

Key             Value
---             -----
serial_number   3746eb17371540d9
signed_key      ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1y...
```

- Установите полученный подписанный сертификат в качестве `HostCertificate` в конфигурации SSH на хост-машине.

```text
$ stronghold write -field=signed_key ssh-host-signer/sign/hostrole \
  cert_type=host \
  public_key=@/etc/ssh/ssh_host_rsa_key.pub > /etc/ssh/ssh_host_rsa_key-cert.pub
```

Установите права доступа к сертификату на `0640`:

```text
chmod 0640 /etc/ssh/ssh_host_rsa_key-cert.pub
```

Добавьте ключ хоста и сертификат хоста в файл конфигурации SSH.

```text
    # /etc/ssh/sshd_config
    # ...

    # For client keys
    TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pem

    # For host keys
    HostKey /etc/ssh/ssh_host_rsa_key
    HostCertificate /etc/ssh/ssh_host_rsa_key-cert.pub
```

Перезапустите службу SSH, чтобы применить изменения.

### Верификация хоста на стороне клиента

- Получите открытый ключ CA хоста для проверки подписи хоста.

```text
curl http://127.0.0.1:8200/v1/ssh-host-signer/public_key
```

```text
stronghold read -field=public_key ssh-host-signer/config/ca
```

- Добавьте полученный открытый ключ в файл `known_hosts`.

```text
# ~/.ssh/known_hosts

@cert-authority *.example.com ssh-rsa AAAAB3NzaC1yc2EAAA...
```

- Можно выполнять вход по SSH на удаленные машины.

## Устранение неполадок

Для облегчения настройки и отладки процесса подписания ключей, включите функцию `VERBOSE` в конфигурации SSH.

```text
# /etc/ssh/sshd_config
# ...
LogLevel VERBOSE
```

Выполните перезапуск SSH после внесения изменений.

По умолчанию SSH ведет журнал в `/var/log/auth.log`, но в нем так же будут записи от других служб. Чтобы извлечь только журналы SSH, выполните следующие действия:

```shell-session
tail -f /var/log/auth.log | grep --line-buffered "sshd"
```

Если вам не удается установить соединение с хостом, логи сервера SSH могут помочь в поиске причин.

### Имя пользователя не входит в список основных пользователей

Если в `auth.log` отображаются следующие сообщения:

```text
# /var/log/auth.log
key_cert_check_authority: invalid certificate
Certificate invalid: name is not a listed principal
```

Сертификат не разрешает использовать имя пользователя в качестве основного имени для аутентификации в системе.
Скорее всего, это связано с ошибкой OpenSSH (подробнее см. в разделе [«Известные проблемы»](#известные-проблемы)).
Эта ошибка не учитывает значение опции `allowed_users`, равное «\*». Вот способы обойти эту проблему:

- Установите роли `default_user`. Если вы всегда аутентифицируетесь под одним и тем же пользователем, установите роль `default_user` на имя пользователя, с которым вы подключаетесь по SSH к удаленной машине:

```text
stronghold write ssh/roles/my-role -<<"EOH"
   {
     "default_user": "YOUR_USER",
     // ...
   }
EOH
```

- Установите `valid_principals` во время подписания. В ситуациях, когда несколько пользователей могут проходить аутентификацию в SSH через Stronghold,
установите, чтобы список основных имен пользователей при подписании ключа включал текущее имя пользователя:

```text
$ stronghold write ssh-client-signer/sign/my-role -<<"EOH"
    {
      "valid_principals": "my-user"
      // ...
    }
EOH
```

### Нет приглашения для ввода команд после входа в систему

Если вы не видите приглашения для ввода команд после аутентификации на хост-машине, возможно, в подписанном сертификате отсутствует расширение `permit-pty`.
Существует два способа добавить это расширение в подписанный сертификат.

- В рамках создания роли

```text
$ stronghold write ssh-client-signer/roles/my-role -<<"EOH"
  {
    "default_extensions": {
      "permit-pty": ""
    }
    // ...
  }
EOH
```

- В рамках самой операции подписи:

```text
$ stronghold write ssh-client-signer/sign/my-role -<<"EOH"
  {
    "extensions": {
      "permit-pty": ""
    }
    // ...
  }
EOH
```

### Нет переадресации портов

Если переадресация портов с удаленного компьютера на хост не работает, возможно, в подписанном сертификате отсутствует расширение `permit-port-forwarding`.
Добавьте расширение в процессе создания или подписи роли, чтобы включить переадресацию портов. Примеры см. в разделе [«Нет приглашения для ввода команд после входа в систему»](#нет-приглашения-для-ввода-команд-после-входа-в-систему).

```json
{
  "default_extensions": {
    "permit-port-forwarding": ""
  }
}
```

### Нет переадресации x11

Если переадресация X11 с удаленного компьютера на хост не работает, возможно, в подписанном сертификате отсутствует расширение `permit-X11-forwarding`.
Добавьте расширение в процессе создания или подписи роли, чтобы включить переадресацию X11. Примеры см. в разделе [«Нет приглашения для ввода команд после входа в систему»](#нет-приглашения-для-ввода-команд-после-входа-в-систему).

```json
{
  "default_extensions": {
    "permit-X11-forwarding": ""
  }
}
```

### Нет переадресации агента

Если переадресация агентов с удаленного компьютера на хост не работает, в подписанном сертификате может отсутствовать расширение `permit-agent-forwarding`.
Добавьте расширение в процессе создания или подписи роли, чтобы включить переадресацию агентов. Примеры см. в разделе [«Нет приглашения для ввода команд после входа в систему»](#нет-приглашения-для-ввода-команд-после-входа-в-систему).

```json
{
  "default_extensions": {
    "permit-agent-forwarding": ""
  }
}
```

### Комментарии для ключа

Если требуется сохранение [атрибутов комментариев](https://www.rfc-editor.org/rfc/rfc4716#section-3.3.2) в ключах, то для этой операции могут быть необходимы дополнительные шаги.
Закрытый и открытый ключи могут иметь комментарии, например, аналогично тому как используется `ssh-keygen` с параметром `-C`:

```shell-session
ssh-keygen -C "...Comments" -N "" -t rsa -b 4096 -f host-ca
```

Значения ключей, содержащие комментарии, должны быть переданы вместе с параметрами, связанными с данным ключем.
Ниже приведены примеры команд с использованием Stronghold CLI и API.

```shell-extension
# Using CLI:
stronghold secrets enable -path=hosts-ca ssh
KEY_PRI=$(cat ~/.ssh/id_rsa | sed -z 's/\n/\\n/g')
KEY_PUB=$(cat ~/.ssh/id_rsa.pub | sed -z 's/\n/\\n/g')
# Create / update keypair in stronghold
stronghold write ssh-client-signer/config/ca \
  generate_signing_key=false \
  private_key="${KEY_PRI}" \
  public_key="${KEY_PUB}"
```

```shell-extension
# Using API:
curl -X POST -H "X-Vault-Token: ..." -d '{"type":"ssh"}' http://127.0.0.1:8200/v1/sys/mounts/hosts-ca
KEY_PRI=$(cat ~/.ssh/id_rsa | sed -z 's/\n/\\n/g')
KEY_PUB=$(cat ~/.ssh/id_rsa.pub | sed -z 's/\n/\\n/g')
tee payload.json <<EOF
{
  "generate_signing_key" : false,
  "private_key"          : "${KEY_PRI}",
  "public_key"           : "${KEY_PUB}"
}
EOF
# Create / update keypair in stronghold
curl -X POST -H "X-Vault-Token: ..." -d @payload.json http://127.0.0.1:8200/v1/hosts-ca/config/ca
```

{% alert level="warning" %}Не добавляйте пароль к закрытому ключу, так как Stronghold не сможет его расшифровать. Уничтожьте открытый и закрытый ключи и `payload.json` с вашего хоста сразу после подтверждения успешной загрузки.
{% endalert %}

### Известные проблемы

- В системах, поддерживающих SELinux, вам может потребоваться настроить связанные типы, чтобы демон SSH мог их читать.
Например, установить для подписанного сертификата хоста тип `sshd_key_t`.

- В некоторых версиях SSH вы можете получить следующую ошибку:

```text
  no separate private key for certificate
```

Эта ошибка появилась в OpenSSH версии 7.2 и была исправлена в версии 7.5. См. [OpenSSH bug 2617](https://bugzilla.mindrot.org/show_bug.cgi?id=2617)

- В некоторых версиях SSH вы можете получить следующую ошибку на хосте:

```text
  userauth_pubkey: certificate signature algorithm ssh-rsa: signature algorithm not supported [preauth]
```

Исправление заключается в добавлении следующей строки в /etc/ssh/sshd_config

```text
  CASignatureAlgorithms ^ssh-rsa
```

Алгоритм ssh-rsa больше не поддерживается в [OpenSSH 8.2](https://www.openssh.com/txt/release-8.2)
