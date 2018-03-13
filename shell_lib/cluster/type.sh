#!/bin/bash -e

# Автоопределятор "типа" кластера Kubernetes.
#
# Тип кластера — это внутреннее понятие Флант. Нам необходимо знать тип кластера, для того, чтобы корректно его настроить и
# правильно поставить в него все компоненты. Например, в AWS нужно особым образом интегрировать nginx c elb и там нет flanneld,
# но нужен автоскейлер. Подобные особенности есть и в других "типах".
#
# По-хорошему можно было бы разделить "способ установки" и "cloud provider", и рассматривать все варианты установки для каждого
# провайдера. Но это сильно усложнит нам работу, а никакого реального результата не даст. Унификация — мать порядка! Поэтому мы
# "искусственно" ограничиваем возможные типы кластеров следующим набором:
#  * AWS — AWS + kops
#  * ACS — Azure + acs-engine
#  * GCE — GCE + kops
#  * Manual — "все остальное" + kubeadm
#
# Дальше возможны следующие изменения:
#  * когда kops научится разворачивать кластер в Azure — мы откажемся от типа ACS в пользу типа Azure
#  * если нам понадобится работать с кластером в GKE — будет тип gke
#  * когда kops дозреет до нормальной установки в VMWare и у нас появится соответствующий кейс — сделаем vmware
#  * когда kops дозреет до работы на железе — заменим Manual на cloudless
#
# Что касается способа детектирования:
#  * есть (вроде бы) нормальный способ определить, что кластер развернут в (и интегрирован с) каком-то облаке
#    — через проверку значения аргумента --cloud-provider у controller'а
#  * точного способа определить "способ установки", которым кластер был поставлен (kops, acs-engine или kubeadm)
#    не удалось найти (есть некоторые "вторичные половые признаки", типа различий в названии лейблов, но считать
#    их надежными и однозначными нельзя)
#  * таким образом мы считаем, что если AWS и GCE — это всегда kops, если Azure — это всегда acs-engine (пока kops
#    не научился, потом придумаем как отличать), а если ничего из этих трех — значит это "без облака" и kubeadm


function cluster::type() {
  if $(kubectl -n kube-system get pod -l k8s-app=kube-controller-manager -o=jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null | grep -- '--cloud-provider=aws' > /dev/null);  then
    echo AWS
  elif $(kubectl -n kube-system get pod -l k8s-app=kube-controller-manager -o=jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null | grep -- '--cloud-provider=gce' > /dev/null); then
    echo GCE
  elif $(kubectl -n kube-system get pod -l component=kube-controller-manager -o=jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null | grep -- '--cloud-provider=azure' > /dev/null);  then
    echo ACS
  else
    echo Manual
  fi
}
