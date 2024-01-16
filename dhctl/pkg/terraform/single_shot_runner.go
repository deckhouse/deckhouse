package terraform

import "sync"

func NewSingleShotRunner(runner *Runner) *SingleShotRunner {
	return &SingleShotRunner{
		Runner: runner,
	}
}

type SingleShotRunner struct {
	*Runner

	init, apply, plan, destroy, stop sync.Once
}

func (r *SingleShotRunner) Init() (err error) {
	r.init.Do(func() {
		err = r.Runner.Init()
	})
	return
}

func (r *SingleShotRunner) Apply() (err error) {
	r.apply.Do(func() {
		err = r.Runner.Apply()
	})
	return
}

func (r *SingleShotRunner) Plan() (err error) {
	r.plan.Do(func() {
		err = r.Runner.Plan()
	})
	return
}

func (r *SingleShotRunner) GetTerraformOutput(output string) ([]byte, error) {
	return r.Runner.GetTerraformOutput(output)
}

func (r *SingleShotRunner) Destroy() (err error) {
	r.destroy.Do(func() {
		err = r.Runner.Destroy()
	})
	return
}

func (r *SingleShotRunner) ResourcesQuantityInState() int {
	return r.Runner.ResourcesQuantityInState()
}

func (r *SingleShotRunner) Stop() {
	r.stop.Do(func() {
		r.Runner.Stop()
	})
}
