package task

import (
	"bytes"
	"fmt"
)

// TODO Тут явно что-то другое должно быть
type UnitConfig struct {
	Name         string
	BeforeHelm   *int
	AfterHelm    *int
	Crontab      *string
	AllowFailure bool
}

type Task struct {
	FailureCount int
	UnitConfig   UnitConfig
}

func NewTask(unitConfig UnitConfig) *Task {
	return &Task{
		FailureCount: 0,
		UnitConfig:   unitConfig,
	}
}

func (t *Task) DumpAsText() string {
	var buf bytes.Buffer
	buf.WriteString(t.UnitConfig.Name)
	if t.FailureCount > 0 {
		buf.WriteString(fmt.Sprintf("failed %d times. ", t.FailureCount))
	}
	if t.UnitConfig.AfterHelm != nil {
		buf.WriteString(fmt.Sprintf("afterHelm order %d ", *t.UnitConfig.AfterHelm))
	}
	if t.UnitConfig.BeforeHelm != nil {
		buf.WriteString(fmt.Sprintf("beforeHelm order %d ", *t.UnitConfig.BeforeHelm))
	}
	return buf.String()
}

func (t *Task) IncrementFailureCount() {
	t.FailureCount++
}
