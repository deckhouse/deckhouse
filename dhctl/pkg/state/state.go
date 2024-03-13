package state

type NodeGroupTerraformState struct {
	State    map[string][]byte
	Settings []byte
}
