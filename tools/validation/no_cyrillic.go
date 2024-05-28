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
	"regexp"
	"strings"
)

var skipDocRe = regexp.MustCompile(`doc-ru-.+\.y[a]?ml$|_RU\.md$|_ru\.html$|docs/site/_.+|docs/documentation/_.+|tools/spelling/.+`)
var skipI18NRe = regexp.MustCompile(`/i18n/`)
var skipSelfRe = regexp.MustCompile(`no_cyrillic(_test)?.go$`)

func RunNoCyrillicValidation(info *DiffInfo, title string, description string) (exitCode int) {
	fmt.Printf("Run 'no cyrillic' validation ...\n")

	exitCode = 0
	if title != "" {
		fmt.Printf("Check title ... ")
		msg, hasCyr := checkCyrillicLetters(title)
		if hasCyr {
			fmt.Printf("ERROR\n%s\n", msg)
			exitCode = 1
		} else {
			fmt.Printf("OK\n")
		}
	}
	if description != "" {
		fmt.Printf("Check description ... ")
		msg, hasCyr := checkCyrillicLetters(description)
		if hasCyr {
			fmt.Printf("ERROR\n%s\n", msg)
			exitCode = 1
		} else {
			fmt.Printf("OK\n")
		}
	}

	fmt.Printf("Check new and updated lines ... ")
	if len(info.Files) == 0 {
		fmt.Printf("OK, diff is empty\n")
	} else {
		fmt.Println("")

		msgs := NewMessages()

		//hasErrors := false
		for _, fileInfo := range info.Files {
			if !fileInfo.HasContent() {
				continue
			}
			// Check only added or modified files
			if !(fileInfo.IsAdded() || fileInfo.IsModified()) {
				continue
			}

			fileName := fileInfo.NewFileName

			if skipDocRe.MatchString(fileName) {
				msgs.Add(NewSkip(fileName, "documentation"))
				continue
			}

			if skipI18NRe.MatchString(fileName) {
				msgs.Add(NewSkip(fileName, "translation file"))
				continue
			}

			if skipSelfRe.MatchString(fileName) {
				msgs.Add(NewSkip(fileName, "self"))
				continue
			}

			// Get added or modified lines
			newLines := fileInfo.NewLines()
			if len(newLines) == 0 {
				msgs.Add(NewSkip(fileName, "no lines added"))
				continue
			}

			cyrMsg, hasCyr := checkCyrillicLettersInArray(newLines)
			if hasCyr {
				msgs.Add(NewError(fileName, "should not contain Cyrillic letters", cyrMsg))
				continue
			}

			msgs.Add(NewOK(fileName))
		}

		msgs.PrintReport()

		if msgs.CountErrors() > 0 {
			exitCode = 1
		}
	}

	return exitCode
}

var cyrRe = regexp.MustCompile(`[А-Яа-яЁё]+`)
var cyrPointerRe = regexp.MustCompile(`[А-Яа-яЁё]`)
var cyrFillerRe = regexp.MustCompile(`[^А-Яа-яЁё]`)

func checkCyrillicLetters(in string) (string, bool) {
	if strings.Contains(in, "\n") {
		return checkCyrillicLettersInArray(strings.Split(in, "\n"))
	}
	return checkCyrillicLettersInString(in)
}

// checkCyrillicLettersInString returns a fancy message if input string contains Cyrillic letters.
func checkCyrillicLettersInString(line string) (string, bool) {
	if !cyrRe.MatchString(line) {
		return "", false
	}

	// Replace all tabs with spaces to prevent shifted cursor.
	line = strings.Replace(line, "\t", "    ", -1)

	// Make string with pointers to Cyrillic letters so user can detect hidden letters.
	cursor := cyrFillerRe.ReplaceAllString(line, "-")
	cursor = cyrPointerRe.ReplaceAllString(cursor, "^")
	cursor = strings.TrimRight(cursor, "-")

	const formatPrefix = "  "

	return formatPrefix + line + "\n" + formatPrefix + cursor, true
}

// checkCyrillicLettersInArray returns a fancy message for each string in array that contains Cyrillic letters.
func checkCyrillicLettersInArray(lines []string) (string, bool) {
	res := make([]string, 0)

	hasCyr := false
	for _, line := range lines {
		msg, has := checkCyrillicLettersInString(line)
		if has {
			hasCyr = true
			res = append(res, msg)
		}
	}

	return strings.Join(res, "\n"), hasCyr
}
