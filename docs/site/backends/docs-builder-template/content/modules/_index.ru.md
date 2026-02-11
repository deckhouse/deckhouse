---
title: Модули Deckhouse
url: modules/
layout: modules-list
---

<p class="tile__descr">Библиотека модулей доступных для использования в Deckhouse.</p>

<p>Используйте фильтр по категориям слева для поиска модулей по их функциональности.</p>


Пример:

```mermaid
graph TB
  linkStyle default fill:#ffffff

  subgraph diagram ["System Context View: Deckhouse Kubernetes Platform"]
    style diagram fill:#ffffff,stroke:#ffffff

    1("<div style='font-weight: bold'>Пользователь</div><div style='font-size: 70%; margin-top: 0px'>[Person]</div><div style='font-size: 80%; margin-top:10px'>Конечный пользователь<br />приложений, запущенных на<br />платформе</div>")
    style 1 fill:#8f7feb,stroke:#8f7feb,color:#ffffff
    10("<div style='font-weight: bold'>IaaS</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>Публичные и частные облака,<br />системы виртуализации</div>")
    style 10 fill:#ededed,stroke:#696a6d,color:#000000
    11("<div style='font-weight: bold'>Внешние\nсистемы хранения логов и SIEM-системы</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>Elasticsearch, Splunk,<br />Logstash, KUMA</div>")
    style 11 fill:#ededed,stroke:#696a6d,color:#000000
    12("<div style='font-weight: bold'>Публичные серверы NTP</div><div style='font-size: 70%; margin-top: 0px'></div>")
    style 12 fill:#ededed,stroke:#696a6d,color:#000000
    13("<div style='font-weight: bold'>Внешние\nContainer Registry</div><div style='font-size: 70%; margin-top: 0px'></div>")
    style 13 fill:#ededed,stroke:#696a6d,color:#000000
    15("<div style='font-weight: bold'>Сетевая инфраструктура</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>Сетевые маршрутизаторы,<br />коммутаторы</div>")
    style 15 fill:#ededed,stroke:#696a6d,color:#000000
    16("<div style='font-weight: bold'>Системы хранения</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>СХД HPE, Huawei, NetApp,<br />YADRO TATLIN</div>")
    style 16 fill:#ededed,stroke:#696a6d,color:#000000
    17("<div style='font-weight: bold'>dhctl CLI</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>Запущенная в инсталляторе<br />Deckhouse</div>")
    style 17 fill:#ededed,stroke:#696a6d,color:#000000
    18("<div style='font-weight: bold'>Deckhouse Kubernetes Platform</div><div style='font-size: 70%; margin-top: 0px'>[Software System]</div><div style='font-size: 80%; margin-top:10px'>Варианты инсталляции для<br />разных типов инфраструктуры</div>")
    style 18 fill:#ffffff,stroke:#004df2,color:#004df2
    2("<div style='font-weight: bold'>Разработчик</div><div style='font-size: 70%; margin-top: 0px'>[Person]</div><div style='font-size: 80%; margin-top:10px'>Пользователь платформы,<br />устанавливает и тестирует<br />приложения</div>")
    style 2 fill:#8f7feb,stroke:#8f7feb,color:#ffffff
    3("<div style='font-weight: bold'>Администратор</div><div style='font-size: 70%; margin-top: 0px'>[Person]</div><div style='font-size: 80%; margin-top:10px'>Инженер, ответственный за<br />установку и эксплуатацию<br />платформы</div>")
    style 3 fill:#8f7feb,stroke:#8f7feb,color:#ffffff
    4("<div style='font-weight: bold'>Инженер безопасности</div><div style='font-size: 70%; margin-top: 0px'>[Person]</div><div style='font-size: 80%; margin-top:10px'>Инженер, ответственный за<br />информационную безопасность</div>")
    style 4 fill:#8f7feb,stroke:#8f7feb,color:#ffffff
    7("<div style='font-weight: bold'>Получатели алертов</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>SMTP, PagerDuty, Slack,<br />Telegram, Webhook</div>")
    style 7 fill:#ededed,stroke:#696a6d,color:#000000
    8("<div style='font-weight: bold'>Внешние провайдеры аутентификации</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>OIDC, LDAP, GitLab, GitHub</div>")
    style 8 fill:#ededed,stroke:#696a6d,color:#000000
    9("<div style='font-weight: bold'>Публичные удостоверяющие центры</div><div style='font-size: 70%; margin-top: 0px'></div><div style='font-size: 80%; margin-top:10px'>Let’s Encrypt, HashiCorp<br />Vault, Venafi</div>")
    style 9 fill:#ededed,stroke:#696a6d,color:#000000

    3-- "<div>Устанавливает и настраивает<br />платформу</div><div style='font-size: 70%'></div>" -->18
    2-- "<div>Использует платформу</div><div style='font-size: 70%'></div>" -->18
    1-- "<div>Использует запущенные<br />приложения</div><div style='font-size: 70%'></div>" -->18
    4-- "<div>Управляет информационной<br />безопасностью</div><div style='font-size: 70%'></div>" -->18
    18-- "<div>Синхронизирует время</div><div style='font-size: 70%'></div>" -->12
    18-- "<div>Управляет ресурсами IaaS</div><div style='font-size: 70%'></div>" -->10
    1-- "<div>Управляет узлами через<br />утилиту dhctl (bootstrap,<br />converge)</div><div style='font-size: 70%'></div>" -->17
    17-- "<div>Управляет ресурсами IaaS по<br />требованию</div><div style='font-size: 70%'></div>" -->10
    17-- "<div>Читает секрет с состоянием<br />Terraform [6443 TCP]</div><div style='font-size: 70%'></div>" -->18
    18-- "<div>Скачивает образы</div><div style='font-size: 70%'></div>" -->13
    18-- "<div>Выполняет аутентификацию<br />пользователей</div><div style='font-size: 70%'></div>" -->8
    18-- "<div>Анонсирует IP-адреса через<br />BGP</div><div style='font-size: 70%'></div>" -->15
    18-- "<div>Отправляет логи и события<br />безопасности</div><div style='font-size: 70%'></div>" -->11
    18-- "<div>Отправляет алерты</div><div style='font-size: 70%'></div>" -->7
    18-- "<div>Выпускает сертификаты</div><div style='font-size: 70%'></div>" -->9
    18-- "<div>Управляет томами</div><div style='font-size: 70%'></div>" -->16
end
```
