# Общие заметки и нюансы

## Общие вводные

- На cm вешается финалайзер который снимает контроллер
- На контроллер вешается финалайзер который снимает d8

## Контроллер

1. Подписываемся
   - на routingTable
   - на ipRule
   - на cm
   - на nodes
2. Если в cm есть DeletionTimestamp и для всех нод в status == "successfully cleared", то снимаем финалайзер с cm.
3. Если в routingTable нет ipRoutingTableID генерируем его и дописываем в status

## Логика агента

1. Подписываемся на cm
2. По приходу изменений
- При создании или изменении или удалении (cm)
  - Ищем в cm.data[hostname]
    - Если нашли и нет DeletionTimestamp, то преобразуем cm.data[hostname] в struct nodeRoutesMap
    - Если не нашли или есть DeletionTimestamp, то создаем пустой struct nodeRoutesMap (заглушка для данных из cm)
  - Получаем все роуты из хостовой системы с realm 216
  - Преобразуем их struct nodeRoutesMap
  - Сравниваем nodeRoutesMap из CM и c ноды
  - Если есть изменения то получаем их
  - Удаляем с ноды более не нужные маршруты
    - И пишем об этом в event
  - Добавляем на ноду отсутствующие маршруты
    - И пишем об этом в event
  - Если есть cm.data[hostname], то пишем (??? запись статуса триггерит обновление cm и так по кругу)
    - в status "successfully applied" или ошибку
    - lastCheckTimestamp текущее время
