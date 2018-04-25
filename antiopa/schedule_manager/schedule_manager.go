package schedule_manager

var (
    ScheduleCh <-chan string
)

type ScheduleManager interface {
    Add(crontab string) (scheduleId string, error)
    Run()
}

func Init() (ScheduleManager, error) {
    return nil, nil
}
