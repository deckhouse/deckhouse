package actions

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"

	"flant/candictl/pkg/log"
)

type ManifestTask struct {
	Name       string
	CreateFunc func(manifest interface{}) error
	UpdateFunc func(manifest interface{}) error
	Manifest   func() interface{}
}

func (task *ManifestTask) Create() error {
	log.InfoF("Manifest for %s\n", task.Name)
	manifest := task.Manifest()

	err := task.CreateFunc(manifest)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("create resource: %v", err)
		}
		log.InfoF("%s already exists. Trying to update ... ", task.Name)
		err = task.UpdateFunc(manifest)
		if err != nil {
			log.ErrorLn("ERROR!")
			return fmt.Errorf("update resource: %v", err)
		}
		log.InfoLn("OK!")
	}
	return nil
}
