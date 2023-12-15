package main

import "testing"

func TestAssembleErrorRegexp(t *testing.T) {
	input := "error building site: assemble: \x1b[1;36m\"/app/hugo/content/modules/moduleName/BROKEN.md:1:1\"\x1b[0m: EOF looking for end YAML front matter delimiter"
	match := assembleErrorRegexp.FindStringSubmatch(input)
	if match == nil || len(match) != 4 {
		t.Fatalf("unexpected match %#v", match)
	}

	path := match[1]
	if path != "/app/hugo/content/modules/moduleName/BROKEN.md" {
		t.Fatalf("unedxpcted path %q", path)
	}
}
