---
title: Файл конфигурации установки
description: Набор YAML-документов с настройками Deckhouse Platform и манифестами объектов кластера, которые создаются после установки. Передаётся CLI-установщику параметром --config.
lang: ru
---

Файл содержит [InitConfiguration](reference/api/cr.html#initconfiguration) (обязательно), [ClusterConfiguration](reference/api/cr.html#clusterconfiguration) (обязательно, кроме установки в существующий кластер), [StaticClusterConfiguration](reference/api/cr.html#staticclusterconfiguration) или конфигурацию кластера провайдера, а также при необходимости [ModuleConfig](reference/api/cr.html#moduleconfig), [NodeGroup](/modules/node-manager/cr.html#nodegroup) и другие объекты. [CLI-установщик](glossary.html#installer) читает его при [preflight-проверках](glossary.html#preflight-checks) и бутстрапе кластера.

Подробнее в разделе [«Установка платформы»](installing/).
