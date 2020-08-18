package actions

import (
	"github.com/flant/logboek"
	"k8s.io/apimachinery/pkg/api/errors"
)

type ManifestTask struct {
	Name       string
	CreateFunc func(manifest interface{}) error
	UpdateFunc func(manifest interface{}) error
	Manifest   func() interface{}
}

func (task *ManifestTask) Create() error {
	logboek.LogInfoF("Manifest for %s\n", task.Name)
	manifest := task.Manifest()

	err := task.CreateFunc(manifest)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		logboek.LogInfoF("%s already exists. Trying to update ... ", task.Name)
		err = task.UpdateFunc(manifest)
		if err != nil {
			logboek.LogWarnLn("ERROR!")
			return err
		}
		logboek.LogInfoLn("OK!")
	}
	return nil
}
