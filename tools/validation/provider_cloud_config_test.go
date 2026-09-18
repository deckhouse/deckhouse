/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import "testing"

func Test_provider_cloud_config_paths(t *testing.T) {
	cases := []struct {
		file    string
		watched bool
	}{
		{"modules/030-cloud-provider-dvp/candi/terraform-modules/master/templates/cloudinit.tftpl", true},
		{"ee/modules/030-cloud-provider-dynamix/candi/terraform-modules/master-node/variables.tf", true},
		{"modules/030-cloud-provider-azure/candi/layouts/standard/master-node/main.tf", true},
		{"modules/040-node-manager/hooks/handler.go", false},
		{"dhctl/pkg/config/nodeusers.go", false},
		{"modules/030-cloud-provider-dvp/candi/openapi/cluster_configuration.yaml", false},
	}

	for _, c := range cases {
		if got := providerTerraformRe.MatchString(c.file); got != c.watched {
			t.Errorf("%s: watched=%v, want %v", c.file, got, c.watched)
		}
	}
}

// The key belongs to dhctl: a second users key in the finished document drops the first,
// and with it the account converge logs in with.
func Test_provider_cloud_config_finds_the_users_key(t *testing.T) {
	cases := []struct {
		line   string
		caught bool
	}{
		{`  "users" : concat(local.base_users, [])`, true},
		{`    "users": local.base_users`, true},
		{"users:", true},
		{"  - users:", true},
		{`  master_cloud_init_script = yamlencode(merge({}, yamldecode(base64decode(var.cloudConfig))))`, true},
		{`  cloud_config = try(jsondecode(base64decode(var.cloudConfig)), tomap({}))`, true},

		// A provider may still add its own keys and name groups of an account it does not
		// declare here; neither takes the list over.
		{"hostname: ${host_name}", false},
		{`  ssh_authorized_keys = [local.ssh_pubkey]`, false},
		{`      "groups" : "users, wheel",`, false},
		{`  user_data = var.cloud_config == "" ? "" : base64decode(var.cloud_config)`, false},
		{`variable "node_users" {`, false},
	}

	for _, c := range cases {
		found := findCloudConfigOwnership([]string{c.line})
		if (found != "") != c.caught {
			t.Errorf("%q: caught=%v, want %v", c.line, found != "", c.caught)
		}
	}
}

func Test_provider_cloud_config_validation(t *testing.T) {
	watched := "modules/030-cloud-provider-dvp/candi/terraform-modules/master/templates/cloudinit.tftpl"

	info := &DiffInfo{Files: []*DiffFileInfo{{
		NewFileName: watched,
		OldFileName: watched,
		Lines:       []string{"+users:", "+- default"},
	}}}

	if code := RunProviderCloudConfigValidation(info); code != 1 {
		t.Errorf("taking over the users key must fail the validation, got exit code %d", code)
	}

	info = &DiffInfo{Files: []*DiffFileInfo{{
		NewFileName: watched,
		OldFileName: watched,
		Lines:       []string{"+hostname: ${host_name}", "+prefer_fqdn_over_hostname: false"},
	}}}

	if code := RunProviderCloudConfigValidation(info); code != 0 {
		t.Errorf("a provider's own keys must pass, got exit code %d", code)
	}
}

// Every provider assembles its own block and appends it; a module that takes the users
// key over is reported wherever it lives.
func Test_provider_cloud_config_has_no_exceptions(t *testing.T) {
	info := &DiffInfo{Files: []*DiffFileInfo{{
		NewFileName: "ee/modules/030-cloud-provider-dynamix/candi/terraform-modules/master-node/variables.tf",
		OldFileName: "ee/modules/030-cloud-provider-dynamix/candi/terraform-modules/master-node/variables.tf",
		Lines:       []string{`+  "users" : concat(local.base_users, [])`},
	}}}

	if code := RunProviderCloudConfigValidation(info); code != 1 {
		t.Errorf("no module may take the users key over, got exit code %d", code)
	}
}
