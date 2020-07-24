package terraform

func ApplyPipeline(r *Runner, extractFn func(r *Runner) (map[string][]byte, error)) (map[string][]byte, error) {
	err := r.Init()
	if err != nil {
		return nil, err
	}

	err = r.Plan()
	if err != nil {
		return nil, err
	}

	err = r.Apply()
	if err != nil {
		return nil, err
	}

	return extractFn(r)
}

func DestroyPipeline(r *Runner) error {
	err := r.Init()
	if err != nil {
		return err
	}

	err = r.Destroy()
	if err != nil {
		return err
	}

	return nil
}

func GetBaseInfraResult(r *Runner) (map[string][]byte, error) {
	cloudDiscovery, err := r.GetTerraformOutput("cloud_discovery_data")
	if err != nil {
		return nil, err
	}

	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"terraformState": tfState,
		"cloudDiscovery": cloudDiscovery,
	}, nil
}

func GetMasterNodeResult(r *Runner) (map[string][]byte, error) {
	masterIPAddressForSSH, err := r.GetTerraformOutput("master_ip_address_for_ssh")
	if err != nil {
		return nil, err
	}

	nodeInternalIP, err := r.GetTerraformOutput("node_internal_ip_address")
	if err != nil {
		return nil, err
	}

	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"terraformState": tfState,
		"masterIPForSSH": masterIPAddressForSSH,
		"nodeInternalIP": nodeInternalIP,
	}, nil
}

func OnlyState(r *Runner) (map[string][]byte, error) {
	tfState, err := r.getState()
	if err != nil {
		return nil, err
	}

	return map[string][]byte{"terraformState": tfState}, nil
}
