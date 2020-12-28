---
title: "Модуль upmeter: настройки"
---

Модуль по умолчанию **включен**.

## Параметры:
* `disabledProbes` – массив строк из названий групп или определенных проб из группы. Названия можно подсмотреть в web-интерфейсе.
  * Пример:

		disabledProbes:
		- "synthetic/api" # отключить отдельную пробу
		- "synthetic/"    # отключить группу проб
		- control-plane   # или без /
* `statusPageAuthDisabled` – выключение авторизации для status-домена.
  * Значение по умолчанию `false`
* `smokeMiniDisabled` – выключение smokeMini.
  * Значение по умолчанию `false`
* `smokeMini`
	* `storageClass` — storageClass для использования при проверке работоспособности дисков.
		* Если не указано — используется StorageClass существующей PVC, а если PVC пока нет — используется или `global.storageClass`, или `global.discovery.defaultStorageClass`, а если и их нет — данные сохраняются в emptyDir.
		* Если указать `false` — будет форсироваться использование emptyDir'а.
	* `ingressClass` — класс ingress-контроллера, который используется для smoke-mini.
		* Опциональный параметр, по умолчанию используется глобальное значение `modules.ingressClass`.
	* `https` — выбираем, какого типа сертификата использовать для smoke-mini.
		* При использовании этого параметра, полностью переопределяются глобальные настройки `global.modules.https`.
		* `mode` — режим работы HTTPS:
			* `Disabled` — в данном режиме smoke-mini будет работать только по http;
			* `CertManager` — smoke-mini будет работать по https и заказывать сертификат с помощью clusterissuer, заданном в параметре `certManager.clusterIssuerName`;
			* `CustomCertificate` — smoke-mini будет работать по https используя сертификат из namespace `d8-system`;
			* `OnlyInURI` — smoke-mini будет работать по http (подразумевая, что перед ним стоит внешний https-балансер, который терминирует https).
		* `certManager`
			* `clusterIssuerName` — указываем, какой ClusterIssuer использовать для smoke-mini (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
				* По умолчанию `letsencrypt`.
		* `customCertificate`
			* `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для smoke-mini (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
				* По умолчанию `false`.
