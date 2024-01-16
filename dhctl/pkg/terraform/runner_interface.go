package terraform

type RunnerInterface interface {
	Init() error
	Apply() error
	Plan() error
	Destroy() error
	Stop()

	ResourcesQuantityInState() int
	GetTerraformOutput(output string) ([]byte, error)
	GetState() ([]byte, error)
	GetStep() string
	GetChangesInPlan() int
	GetPlanDestructiveChanges() *PlanDestructiveChanges
	GetPlanPath() string
	GetTerraformExecutor() Executor
}
