package terraform

import (
	"testing"
)

func TestPipeline(t *testing.T) {
	fakeTfRunner := FakeRunner{}

	tests := []struct {
		name    string
		test    func() error
		wantErr bool
	}{
		{
			"BasePipeline run",
			func() error {
				basePipeline := Pipeline{
					Step:            "tf_base",
					TerraformRunner: &fakeTfRunner,
					GetResult:       GetBasePipelineResult,
				}
				_, err := basePipeline.Run()
				return err
			},
			false,
		},
		{
			"MasterPipeline run",
			func() error {
				basePipeline := Pipeline{
					Step:            "tf_master",
					TerraformRunner: &fakeTfRunner,
					GetResult:       GetMasterPipelineResult,
				}
				_, err := basePipeline.Run()
				return err
			},
			false,
		},
	}

	for _, tc := range tests {
		err := tc.test()

		if err != nil && !tc.wantErr {
			t.Errorf("%s: %v", tc.name, err)
		}

		if err == nil && tc.wantErr {
			t.Errorf("%s: expected error, didn't get one", tc.name)
		}
	}
}
