---
title: "Модуль runtime-audit-engine: примеры"
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
