package detach

import (
	"context"
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type DetacherOptions struct {
	DetachResources
	OnCheckResult func(*check.CheckResult) error
}

type Detacher struct {
	DetacherOptions

	AgentModuleName string
	SSHClient       *ssh.Client
	Checker         *check.Checker
}

type DetachResources struct {
	Template string
	Values   map[string]any
}

func NewDetacher(checker *check.Checker, sshClient *ssh.Client, opts DetacherOptions) *Detacher {
	return &Detacher{
		DetacherOptions: opts,
		SSHClient:       sshClient,
		Checker:         checker,
	}
}

func (op *Detacher) Detach(ctx context.Context) error {
	checkRes, err := op.Checker.Check(ctx)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	if op.OnCheckResult != nil {
		if err := op.OnCheckResult(checkRes); err != nil {
			return err
		}
	}

	kubeCl, err := op.Checker.GetKubeClient()
	if err != nil {
		return fmt.Errorf("unable to get kube client: %w", err)
	}

	detachResources, err := template.ParseResourcesContent(
		op.DetachResources.Template,
		op.DetachResources.Values,
	)
	if err != nil {
		return fmt.Errorf("unable to parse resources to detach: %w", err)
	}

	if err := resources.DeleteResourcesLoop(ctx, kubeCl, detachResources); err != nil {
		return fmt.Errorf("unable to detach resources: %w", err)
	}

	if err := commander.DeleteManagedByCommanderConfigMap(ctx, kubeCl); err != nil {
		return fmt.Errorf("unable to remove commander ConfigMap: %w", err)
	}

	return nil
}
