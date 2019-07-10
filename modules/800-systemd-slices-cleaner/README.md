Модуль systemd-slices-cleaner
=============================

Есть проблема в systemd, используемом в Ubuntu 16.04, и возникает она при управлении mount’ами, которые создаются для подключения subPath из ConfigMap’ов или secret’ов. После завершения работы pod’а сервис systemd и его служебный mount остаются в системе. Со временем их накапливается огромное количество. На эту тему даже есть issues:

kops #5916 https://github.com/kubernetes/kops/issues/5916
kubernetes #57345 https://github.com/kubernetes/kubernetes/issues/57345

… в последнем из которых ссылаются на PR в systemd: #7811 https://github.com/systemd/systemd/pull/7811 (issue в systemd — #7798 https://github.com/systemd/systemd/issues/7798).

Проблемы уже нет в Ubuntu 18.04
### Как работает

Каждый час рандомно в течение получаса запускается и останавливает более ненужные systemd-слайсы на нодах, отмеченных лейблом *systemd-slices-cleaner.antiopa.flant.com/enabled=true*.

### Что нужно настроить?

Настраивать ничего не нужно, все работает само по себе.
