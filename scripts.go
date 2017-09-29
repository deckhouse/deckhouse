package main

var (
  // нельзя делать fetch и run одновременно
  runMutex
  
  // отправляем событие, что изменился коммит в репо или был новый git clone
  // какие конкретно изменения произошли не разбираем
  ScriptsUpdated chan bool
)

func FetchScripts(Repo map[string]string) {
  // todo: git clone или fetch + смотрим изменение коммита, шлем сигнал в ScriptsUpdated
}

func RunScripts(Modules []map[string]string) {
  // todo: запускаем модули
}