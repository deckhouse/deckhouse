/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"io/fs"
	"testing"
)

func TestTemplatesExists(t *testing.T) {
	count := 0

	fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Errorf("walk error: %v", err)
		}

		if d.IsDir() {
			return nil
		}

		t.Logf("- %v", path)

		count++

		return nil
	})

	t.Logf("Templates found: %v", count)

	if count == 0 {
		t.Errorf("no templates found")
	}
}

func TestStaticPodManifest(t *testing.T) {
	model := templateModel{
		Address: "192.168.0.1",
	}

	buf, err := renderTemplate(registryStaticPodTemplateName, &model)
	if err != nil {
		t.Errorf("Cannot load template: %v", err)
	}

	size := len(buf)
	if size == 0 {
		t.Error("Template content is empty!")
	}

	t.Logf("Result:\n%s", buf)
}

func TestDistributionConfig(t *testing.T) {
	model := templateModel{
		Address: "192.168.0.1",
	}

	model.Registry = RegistryConfig{
		UserRO: User{
			Name:         "ro-user",
			Password:     "ro-password",
			PasswordHash: "ro-password-hash",
		},
		UserRW: User{
			Name:         "rw-user",
			Password:     "rw-password",
			PasswordHash: "rw-password-hash",
		},
		Mirrorer: &Mirrorer{
			UserPuller: User{
				Name:         "puller-user",
				Password:     "puller-password",
				PasswordHash: "puller-password-hash",
			},
			UserPusher: User{
				Name:         "pusher-user",
				Password:     "pusher-password",
				PasswordHash: "pusher-password-hash",
			},
		},
	}

	buf, err := renderTemplate(distributionConfigTemplateName, &model)
	if err != nil {
		t.Errorf("Cannot load template: %v", err)
	}

	size := len(buf)
	if size == 0 {
		t.Error("Template content is empty!")
	}

	t.Logf("Result:\n%s", buf)
}

func TestAuthConfigWithMirrorer(t *testing.T) {
	model := templateModel{
		Address: "192.168.0.1",
	}

	model.Registry = RegistryConfig{
		UserRO: User{
			Name:         "ro-user",
			Password:     "ro-password",
			PasswordHash: "ro-password-hash",
		},
		UserRW: User{
			Name:         "rw-user",
			Password:     "rw-password",
			PasswordHash: "rw-password-hash",
		},
		Mirrorer: &Mirrorer{
			UserPuller: User{
				Name:         "puller-user",
				Password:     "puller-password",
				PasswordHash: "puller-password-hash",
			},
			UserPusher: User{
				Name:         "pusher-user",
				Password:     "pusher-password",
				PasswordHash: "pusher-password-hash",
			},
		},
	}

	buf, err := renderTemplate(authConfigTemplateName, &model)
	if err != nil {
		t.Errorf("Cannot load template: %v", err)
	}

	size := len(buf)
	if size == 0 {
		t.Error("Template content is empty!")
	}

	t.Logf("Result:\n%s", buf)

}

func TestAuthConfig(t *testing.T) {
	model := templateModel{
		Address: "192.168.0.1",
	}

	model.Registry = RegistryConfig{
		UserRO: User{
			Name:         "ro-user",
			Password:     "ro-password",
			PasswordHash: "ro-password-hash",
		},
		UserRW: User{
			Name:         "rw-user",
			Password:     "rw-password",
			PasswordHash: "rw-password-hash",
		},
	}

	buf, err := renderTemplate(authConfigTemplateName, &model)
	if err != nil {
		t.Errorf("Cannot load template: %v", err)
	}

	size := len(buf)
	if size == 0 {
		t.Error("Template content is empty!")
	}

	t.Logf("Result:\n%s", buf)
}

func TestMirrorerConfig(t *testing.T) {
	model := templateModel{
		Address: "192.168.0.1",
	}

	model.Registry.Mirrorer = &Mirrorer{
		UserPuller: User{
			Name:         "puller",
			Password:     "puller password",
			PasswordHash: "AS:DLASDLAJSDASD",
		},
		UserPusher: User{
			Name:         "pusher",
			Password:     "pusher password",
			PasswordHash: "AS:DLASDLAJSDASD",
		},
		Upstreams: []string{
			"one",
			"two",
			"three",
		},
	}

	buf, err := renderTemplate(mirrorerConfigTemplateName, &model)
	if err != nil {
		t.Errorf("Cannot load template: %v", err)
	}

	size := len(buf)
	if size == 0 {
		t.Error("Template content is empty!")
	}

	t.Logf("Result:\n%s", buf)
}
