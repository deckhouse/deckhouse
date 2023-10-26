---
title: "Модуль runtime-audit-engine: примеры"
description: Примеры использования модуля runtime-audit-engine Deckhouse.
---

## Добавление одного правила

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: ownership-permissions
spec:
  rules:
  - macro:
      name: spawned_process
      condition: (evt.type in (execve, execveat) and evt.dir=<)
  - rule:
      name: Detect Ownership Change
      desc: detect file permission/ownership change
      condition: >
        spawned_process and proc.name in (chmod, chown) and proc.args contains "/tmp/"
      output: >
        The file or directory below has had its permissions or ownership changed (user=%user.name
        command=%proc.cmdline file=%fd.name parent=%proc.pname pcmdline=%proc.pcmdline gparent=%proc.aname[2])
      priority: Warning
      tags: [filesystem]
```

## Добавление двух правил с макросом и списком

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: nginx-unexpected-port
spec:
  rules:
  - macro:
      name: container
      condition: (container.id != host)

  - macro:
      name: inbound
      condition: >
        (((evt.type in (accept,listen) and evt.dir=<)) or
        (fd.typechar = 4 or fd.typechar = 6) and
        (fd.ip != "0.0.0.0" and fd.net != "127.0.0.0/8") and (evt.rawres >= 0 or evt.res = EINPROGRESS))

  - macro:
      name: outbound
      condition: >
        (((evt.type = connect and evt.dir=<)) or
        (fd.typechar = 4 or fd.typechar = 6) and
        (fd.ip != "0.0.0.0" and fd.net != "127.0.0.0/8") and (evt.rawres >= 0 or evt.res = EINPROGRESS))

  - macro:
      name: app_nginx
      condition: container and container.image contains "nginx"

  - rule:
      name: Unauthorized process opened an outbound connection (nginx)
      desc: nginx process tried to open an outbound connection and is not whitelisted
      condition: outbound and evt.rawres >= 0 and app_nginx
      output: |-
        Non-whitelisted process opened an outbound connection (command=%proc.cmdline connection=%fd.name)
      priority: Warning

  - list:
      name: nginx_allowed_inbound_ports_tcp
      items: [80, 443, 8080, 8443]

  - rule:
      name: Unexpected inbound TCP connection nginx
      desc: detect inbound traffic to nginx using tcp on a port outside of expected set
      condition: |
        inbound and evt.rawres >= 0 and not fd.sport in (nginx_allowed_inbound_ports_tcp) and app_nginx
      output: |-
        Inbound network connection to nginx on unexpected port
        (command=%proc.cmdline pid=%proc.pid connection=%fd.name sport=%fd.sport user=%user.name %container.info image=%container.image)
      priority: Notice
```

## Добавление правила для отправки уведомлений о запуске shell-оболочки в контейнере

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: run-shell-in-container
spec:
  rules:
  - macro: 
      name: container
      condition: container.id != host
  
  - macro: 
      name: spawned_process
      condition: evt.type = execve and evt.dir=<
  
  - rule: 
      name: run_shell_in_container
      desc: a shell was spawned by a non-shell program in a container. Container entrypoints are excluded.
      condition: container and proc.name = bash and spawned_process and proc.pname exists and not proc.pname in (bash, docker)
      output: "Shell spawned in a container other than entrypoint (user=%user.name container_id=%container.id container_name=%container.name shell=%proc.name parent=%proc.pname cmdline=%proc.cmdline)"
      priority: Warning
```

## Дополнительные примеры

Если вам необходимо больше примеров правил, изучите следующие ресурсы:

- [falco rules repository](https://github.com/falcosecurity/rules/blob/32b635394c40a56f8bdeb334c60a46e2edd9908c/rules/application_rules.yaml);
- [artifacthub falco rules](https://artifacthub.io/packages/search?kind=1&sort=relevance&page=1).
