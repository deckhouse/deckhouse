---
title: Что делать при наличии проблем с обновлением DKP?
permalink:
lang: ru
---

Если обновление Deckhouse Kubernetes Platform не проходит, один или несколько подов Deckhouse в пространстве имен `d8-system` находятся в нерабочем состоянии, выполните следующие действия:

1. Проверьте логи Deckhouse с помощью команды:

   ```shell
   d8 k -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
   ```

   При наличии проблем информация о них будет в выводе. При анализе логов особое внимание обращайте на предупреждения (`WARNING`) и сообщения об ошибках (`ERROR`).

1. Проверьте события подов Deckhouse с помощью команды:

   ```shell
   d8 k -n d8-system describe po -l app=deckhouse | awk '
   /^Name:/ { 
       pod = $2; 
       print "=== " pod " ==="; 
       in_events = 0 
   }
   /Events:/ { 
       in_events = 1; 
       next 
   }
   in_events && /^$/ { 
       in_events = 0; 
       print "---" 
   }
   in_events && !/^Events:/ { 
       print $0 
   }
   ' | sed '/^---$/N;/^\n$/D'
   ```

   В событиях подов содержится ключевая информация о проблемах (например, об ошибках планирования, загрузки образов и т.д.). При анализе событий особое внимание обращайте на те, у которых тип `Warning`.

   Пример вывода:

   ```console
   Type     Reason     Age                      From     Message
   ----     ------     ----                     ----     -------
   Warning  Unhealthy  4m44s (x1918 over 154m)  kubelet  Readiness probe failed: HTTP probe failed with statuscode: 500
   ```
