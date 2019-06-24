package registry_watcher

import (
	_ "net/http/pprof"
	"os"

	"github.com/romana/rlog"

	"github.com/deckhouse/deckhouse/antiopa/docker_registry_manager"
	"github.com/deckhouse/deckhouse/antiopa/kube_helper"

	operator "github.com/flant/addon-operator/pkg/addon-operator"
)

var (
	// Имя хоста совпадает с именем пода. Можно использовать для запросов API
	Hostname string

	// registry manager — watch for antiopa image updates
	RegistryManager docker_registry_manager.DockerRegistryManager

	// chan for stopping ManagersEventsHandler infinite loop
	ManagersEventsHandlerStopCh chan struct{}
)

// Собрать настройки - директории, имя хоста, файл с дампом, namespace для tiller
// Проинициализировать все нужные объекты: helm, registry manager, module manager,
// kube events manager
// Создать пустую очередь с заданиями.
func Init() error {
	rlog.Debug("ANTIOPA: Start init")

	var err error

	Hostname, err = os.Hostname()
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot get pod name from hostname: %s", err)
		return err
	}
	rlog.Infof("Antiopa hostname: %s", Hostname)

	// Инициализация слежения за образом
	// TODO Antiopa может и не следить, если кластер заморожен?
	RegistryManager, err = docker_registry_manager.Init(Hostname)
	if err != nil {
		rlog.Errorf("MAIN Fatal: Cannot initialize registry manager: %s", err)
		return err
	}

	return nil
}

// Run запускает все менеджеры, обработчик событий от менеджеров и обработчик очереди.
// Основной процесс блокируется for-select-ом в обработчике очереди.
func Run() {
	rlog.Info("ANTIOPA MAIN: run main loop")

	if RegistryManager != nil {
		// менеджеры - отдельные go-рутины, посылающие события в свои каналы
		RegistryManager.SetErrorCallback(func() {
			operator.MetricsStorage.SendCounterMetric("antiopa_registry_errors", 1.0, map[string]string{})
			return
		})
		go RegistryManager.Run()
	}

	// обработчик событий от менеджеров — события превращаются в таски и
	// добавляются в очередь
	go ManagersEventsHandler()
}

func ManagersEventsHandler() {
	for {
		select {
		// Образ antiopa изменился, нужен рестарт деплоймента (можно и не выходить)
		case newImageId := <-docker_registry_manager.ImageUpdated:
			rlog.Infof("EVENT ImageUpdated")
			err := kube_helper.KubeUpdateDeployment(newImageId)
			if err == nil {
				rlog.Infof("KUBE deployment update successful, exiting ...")
				os.Exit(1)
			} else {
				rlog.Errorf("KUBE deployment update error: %s", err)
			}

		case <-ManagersEventsHandlerStopCh:
			rlog.Infof("EVENT Stop")
			return
		}
	}
}
