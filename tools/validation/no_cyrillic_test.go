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
