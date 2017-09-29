package main

var (
  // отправляем событие, что изменился коммит в репо или был новый git clone
  // какие конкретно изменения произошли не разбираем
  // (старый-коммит, новый-коммит)
  // старый-коммит может быть пустой строкой -- новый clone
  ScriptsUpdated chan (string, string)
)

func FetchScripts(Repo map[string]string) {
  // todo: git clone или fetch + смотрим изменение коммита, шлем сигнал в ScriptsUpdated
}

func InitScriptsManager() {
    
}

// Запускается в отдельной goroutine
func RunScriptsManager() {
    for ;; {
        // Ловим RepoUpdated -> запускаем FetchScripts
    }
}