/*
Copyright 2021 Flant JSC

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

import (
	"fmt"
	"strings"
	"testing"
)

func Test_found_msg(t *testing.T) {
	// Simple check with one Cyrillic letter.
	in := "fooБfoo"
	expected := `  fooБfoo
  ---^`

	actual, has := checkCyrillicLetters(in)

	if !has {
		t.Errorf("Should detect cyrillic letters in string")
	}

	if actual != expected {
		t.Errorf("Expect '%s', got '%s'", expected, actual)
	}

	// No Cyrillic letters.
	in = "asdqwe 123456789 !@#$%^&*( ZXCVBNM"
	expected = ""
	actual, has = checkCyrillicLetters(in)

	if has {
		t.Errorf("Should not detect cyrillic letters in string")
	}

	if actual != expected {
		t.Errorf("Expect '%s', got '%s'", expected, actual)
	}

	// Multiple words with Cyrillic letters.
	in = "asdqwe Там на qw q cheсk tеst qwd неведомых qqw"
	expected =
		"  asdqwe Там на qw q cheсk tеst qwd неведомых qqw\n" +
			"  -------^^^-^^---------^---^-------^^^^^^^^^"

	actual, has = checkCyrillicLetters(in)

	if !has {
		t.Errorf("Should detect cyrillic letters in string")
	}

	if actual != expected {
		fmt.Printf("  %s\n%s\n",
			strings.Repeat("0123456789", len(actual)/2/10+1),
			actual)
		t.Errorf("Expect \n%s\n, got \n%s\n", expected, actual)
	}

	// Multiple messages for string with '\n'.
	in = "Lorem ipsum dolor sit amet,\n consectetur adipiscing elit,\n" +
		"раскрою перед вами всю \nкартину и разъясню," +
		"Ut enim ad minim veniam,"
	expected =
		"  раскрою перед вами всю \n" +
			"  ^^^^^^^-^^^^^-^^^^-^^^\n" +
			"  картину и разъясню,Ut enim ad minim veniam,\n" +
			"  ^^^^^^^-^-^^^^^^^^"

	actual, has = checkCyrillicLetters(in)

	if !has {
		t.Errorf("Should detect cyrillic letters in string")
	}

	if actual != expected {
		fmt.Printf("  %s\n%s\n",
			strings.Repeat("0123456789", len(actual)/2/10+1),
			actual)
		t.Errorf("Expect \n%s\n, got \n%s\n", expected, actual)
	}

}

func Test_skip_localized_role_metadata(t *testing.T) {
	for path, skip := range map[string]bool{
		"modules/101-cert-manager/templates/rbacv2/use/view.yaml":               true,
		"ee/modules/110-istio/templates/rbacv2-compat/legacy-role-aliases.yaml": true,
		"modules/101-cert-manager/rbac.yaml":                                    true,
		"ee/modules/030-cloud-provider-openstack/rbac.yaml":                     true,
		"ee/be/modules/350-node-local-dns/rbac.yaml":                            true,
		"ee/se-plus/modules/015-admission-policy-engine/rbac.yaml":              true,
		"modules/101-cert-manager/templates/rbac-to-us.yaml":                    false,
		"modules/101-cert-manager/openapi/rbac.yaml":                            false,
		"rbac.yaml": false,
	} {
		if got := skipRBACv2Re.MatchString(path); got != skip {
			t.Errorf("%s: skip = %v, want %v", path, got, skip)
		}
	}
}

func Test_skip_russian_sections(t *testing.T) {
	const russianSections = "## Description\n" +
		"Описание изменений.\n" +
		"### Детали\n" +
		"Подробности.\n" +
		"```text\n" +
		"## Checklist\n" +
		"```\n" +
		"Текст после блока кода.\n" +
		"\n" +
		"## Why do we need it, and what problem does it solve?\n" +
		"<!--- This is the most important paragraph. -->\n" +
		"Зачем это нужно.\n" +
		"\n" +
		"## Why do we need it in the patch release (if we do)?\n" +
		"Не обязательно.\n"
	const checklist = "\n## Checklist\n" +
		"- [x] The code is covered by unit tests.\n"
	const changes = "\n## Changelog entries\n" +
		"```changes\n" +
		"section: ci\n" +
		"type: chore\n" +
		"summary: Check commit authors.\n" +
		"```\n"

	for name, tc := range map[string]struct {
		in     string
		hasCyr bool
	}{
		"Russian only in the Russian sections": {
			in: russianSections + checklist + changes,
		},
		"CRLF line endings": {
			in: strings.ReplaceAll(russianSections+checklist+changes, "\n", "\r\n"),
		},
		"Russian in the checklist": {
			in:     russianSections + strings.Replace(checklist, "The code", "Код", 1) + changes,
			hasCyr: true,
		},
		"Russian in the changes block": {
			in:     russianSections + checklist + strings.Replace(changes, "Check commit authors.", "Проверка авторов.", 1),
			hasCyr: true,
		},
		"changes block inside a Russian section": {
			in:     "## Description\nОписание.\n```changes\nsummary: Проверка авторов.\n```\nЕще описание.\n" + checklist,
			hasCyr: true,
		},
		"Russian before any heading": {
			in:     "Описание без шаблона.\n" + russianSections,
			hasCyr: true,
		},
		"Russian under an unknown heading": {
			in:     russianSections + "## Notes\nЗаметки.\n",
			hasCyr: true,
		},
		"translated heading": {
			in:     strings.Replace(russianSections, "## Description", "## Описание", 1) + checklist + changes,
			hasCyr: true,
		},
		"Russian added to a heading": {
			in:     strings.Replace(russianSections, "## Description", "## Description (описание)", 1) + checklist + changes,
			hasCyr: true,
		},
	} {
		if _, has := checkCyrillicLetters(skipRussianSections(tc.in)); has != tc.hasCyr {
			t.Errorf("%s: has Cyrillic = %v, want %v\n%s", name, has, tc.hasCyr, skipRussianSections(tc.in))
		}
	}
}
