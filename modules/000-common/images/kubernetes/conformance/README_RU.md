# Соответствие требованиям CNCF Kubernetes (Sonobuoy)

В этом каталоге для каждой минорной версии Kubernetes хранятся артефакты **сертификационного тестирования на соответствие требованиям**: `e2e.log` и `junit_01.xml`, созданные [Sonobuoy](https://github.com/vmware-tanzu/sonobuoy) в режиме `certified-conformance`.

Используйте эту инструкцию, когда необходимо обновить сохранённые результаты после обновления или проверки поддерживаемой линейки версий Kubernetes.

---

## 1. Подготовка кластера и Deckhouse

Разверните кластер с Deckhouse на целевой версии Kubernetes и убедитесь, что `kubectl` настроен для работы с ним.

---

## 2. Настройка RBAC и admission-контроля (необходимо для корректного запуска)

Pod Security Standards могут блокировать рабочие нагрузки, необходимые набору e2e-тестов. На время запуска ослабьте настройки **Admission Policy Engine** по умолчанию, чтобы ограничения безопасности не мешали выполнению тестов.

Примените следующую конфигурацию один раз (проверьте её перед применением в production-кластере):

```bash
kubectl apply -f - <<'EOF'
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: admission-policy-engine
spec:
  enabled: true
  settings:
    podSecurityStandards:
      defaultPolicy: Privileged
  version: 1
EOF
```

---

## 3. Установка CLI Sonobuoy

Скачайте нужную версию со [страницы релизов Sonobuoy](https://github.com/vmware-tanzu/sonobuoy/releases), распакуйте бинарный файл и добавьте его в `PATH` либо запускайте из каталога распаковки.

Пример для Linux `amd64` (замените версию на необходимую):

```bash
curl -sL -o sonobuoy.tgz \
  'https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.57.3/sonobuoy_0.57.3_linux_amd64.tar.gz'
tar -xzf sonobuoy.tgz sonobuoy
chmod +x sonobuoy
```

---

## 4. Запуск conformance-тестов

```bash
./sonobuoy run --mode=certified-conformance --kubeconfig=/etc/kubernetes/super-admin.conf
```

Дождитесь, пока команда `./sonobuoy status` сообщит о состоянии **completed**. Для наблюдения за ходом выполнения проверяйте создание и удаление пространств имён: тесты запускаются в изолированных пространствах имён.

Обычно выполнение занимает около **2 часов**.

---

## 5. Получение только `e2e.log` и `junit_01.xml`

На машине, где запускается CLI, выполните:

```bash
./sonobuoy retrieve . -f sb.tar.gz
tar -xzf sb.tar.gz \
  plugins/e2e/results/global/e2e.log \
  plugins/e2e/results/global/junit_01.xml
```

В результате будут получены файлы:

`plugins/e2e/results/global/e2e.log`  
`plugins/e2e/results/global/junit_01.xml`

После завершения архив можно удалить: `rm -f sb.tar.gz`.
