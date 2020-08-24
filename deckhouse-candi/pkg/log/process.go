package log

import (
	"fmt"

	"github.com/flant/logboek"
)

func BootstrapProcess(name string, task func() error) error {
	return logboek.LogProcess(fmt.Sprintf("⛵ ~ Bootstrap: %s", name), MainProcessOptions(), task)
}

func ConvergeProcess(name string, task func() error) error {
	return logboek.LogProcess(fmt.Sprintf("🛸 ~ Converge: %s", name), ConvergeOptions(), task)
}

func TerraformProcess(name string, task func() error) error {
	return logboek.LogProcess(fmt.Sprintf("🌱 ~ Terraform: %s", name), TerraformOptions(), task)
}

func CommonProcess(name string, task func() error) error {
	return logboek.LogProcess(fmt.Sprintf("\U0001FA81 ~ Common: %s", name), TaskOptions(), task)
}

func BoldProcess(name string, task func() error) error {
	return logboek.LogProcess(name, BoldOptions(), task)
}

func TerraformBlock(name string, task func() error) error {
	return logboek.LogProcess(name, TerraformOptions(), task)
}
