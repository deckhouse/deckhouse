package docker_registry_manager

import (
	"regexp"
	"testing"

	"github.com/docker/distribution/reference"
)

// https://docs.docker.com/engine/reference/commandline/tag/#extended-description
//
// A tag name must be valid ASCII and may contain lowercase and uppercase letters,
// digits, underscores, periods and dashes. A tag name may not start with a period
//  or a dash and may contain a maximum of 128 characters.

var re = regexp.MustCompile(`(?:([a-zA-Z][a-zA-Z\d\.\-]+(?::\d+)?)\/)?((?:[a-z\d][a-z\d\.\-\_]+\/?)+)(?::([a-zA-Z\d\_][a-zA-Z\d\-\_\.]{0,127}))?`)

var mustPass = []string{
	"localhost/antiopa:latest",
	"localhost/sys/antiopa:latest",
	"antiopa:latest",
	"sys/antiopa:latest",
	"localhost:5000/antiopa",
	"localhost:5000/antiopa:master",
	"localhost:5000/sys/antiopa:master",
	"registry.flant.com/sys/antiopa:master",
	"registry.flant.com/sys/antiopa:stable",
}

var mustFail = []string{
	"",
	"",
}

func TestRegistryImageRegexp(t *testing.T) {
	for _, test := range append(mustPass) {
		matches := re.FindStringSubmatch(test)
		PrintMatches(t, test, matches)
	}
	// uncomment to see output in vscode
	//t.Fail()
}

func PrintMatches(t *testing.T, test string, matches []string) {
	t.Log(test)
	if matches == nil {
		t.Log("  No matches")
		return
	}
	for i, m := range matches {
		t.Logf("  %2d: %s\n", i, m)
	}
}

func TestDockerNormalize(t *testing.T) {
	for _, test := range append(mustPass) {
		ParseByDocker(t, test)
	}
	// uncomment to see output in vscode
	//t.Fail()
}

func ParseByDocker(t *testing.T, test string) {
	t.Log(test)
	distributionRef, err := reference.ParseNormalizedNamed(test)
	t.Logf("  distributionRef: %+v", distributionRef)
	switch {
	case err != nil:
		t.Logf("  err: %v", err)
	case reference.IsNameOnly(distributionRef):
		distributionRef = reference.TagNameOnly(distributionRef)
	}

	tag := ""
	if tagged, ok := distributionRef.(reference.Tagged); ok {
		tag = tagged.Tag()
	}

	//repoInfo := reference.TrimNamed(distributionRef)

	t.Logf("  %s %s %s", reference.Domain(distributionRef), reference.Path(distributionRef), tag)
}

/*
package main_test

import (
	"fmt"
	"regexp"
)

var re = regexp.MustCompile(`https?:\/\/((([^:@]*):?([^@]*))@)?[^@].*`)


var mustPass = []string{
"http://admin:pass@domain.com",
"https://admin:pass@domain.com",
"https://admin@domain.com",
"https://admin:@domain.com",
"https://:pass@domain.com",
"https://domain.com",

}

var mustFail = []string{
"htt://admin:admin@domain.com",
"http://",
}


func main() {

	for _, test := range append(mustPass) {
		matches := re.FindStringSubmatch(test)
		PrintMatches(test, matches)
	}
	for _, test := range append(mustFail) {
		matches := re.FindStringSubmatch(test)
		PrintMatches(test, matches)
	}
}

func PrintMatches(test string, matches []string){
  fmt.Println(test)
  if matches == nil {
    fmt.Println("  No matches")
    return
  }
  for i, m := range matches {
    fmt.Printf("  %2d: %s\n", i, m)
  }
  fmt.Println()
}

*/
