---
title: "Модуль linstor: настройки"
---

<div class="docs__information warning active">
Модуль находится в процессе активного развития, и его функциональность может существенно измениться.
</div>

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:
```
data:
  linstorEnabled: "true"
```
