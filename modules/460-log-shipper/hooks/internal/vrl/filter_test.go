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

package vrl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterInRule(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		res    string
	}{
		{
			name:   "Single value",
			values: []string{"test-1"},
			res: strings.TrimSpace(`
if is_boolean(.parsed_data.test) || is_float(.parsed_data.test) {
    data, err = to_string(.parsed_data.test);
    if err != null {
        false;
    } else {
        includes(["test-1"], data);
    };
} else if .parsed_data.test == null {
    false;
} else {
    includes(["test-1"], .parsed_data.test);
}
`),
		},
		{
			name:   "Two values",
			values: []string{"test-1", "test-2"},
			res: strings.TrimSpace(`
if is_boolean(.parsed_data.test) || is_float(.parsed_data.test) {
    data, err = to_string(.parsed_data.test);
    if err != null {
        false;
    } else {
        includes(["test-1","test-2"], data);
    };
} else if .parsed_data.test == null {
    false;
} else {
    includes(["test-1","test-2"], .parsed_data.test);
}
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := FilterInRule.Render(Args{"filter": map[string]interface{}{
				"Field": "parsed_data.test", "Values": tc.values,
			}})
			require.NoError(t, err)
			require.Equal(t, tc.res, res)
		})
	}
}

func TestFilterNotInRule(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		res    string
	}{
		{
			name:   "Single value",
			values: []string{"test-1"},
			res: strings.TrimSpace(`
if is_boolean(.parsed_data.test) || is_float(.parsed_data.test) {
    data, err = to_string(.parsed_data.test);
    if err != null {
        true;
    } else {
        !includes(["test-1"], data);
    };
} else if .parsed_data.test == null {
    false;
} else {
    !includes(["test-1"], .parsed_data.test);
}
`),
		},
		{
			name:   "Two values",
			values: []string{"test-1", "test-2"},
			res: strings.TrimSpace(`
if is_boolean(.parsed_data.test) || is_float(.parsed_data.test) {
    data, err = to_string(.parsed_data.test);
    if err != null {
        true;
    } else {
        !includes(["test-1","test-2"], data);
    };
} else if .parsed_data.test == null {
    false;
} else {
    !includes(["test-1","test-2"], .parsed_data.test);
}
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := FilterNotInRule.Render(Args{"filter": map[string]interface{}{
				"Field": "parsed_data.test", "Values": tc.values,
			}})
			require.NoError(t, err)
			require.Equal(t, tc.res, res)
		})
	}
}

func TestFilterRegexRule(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		res    string
	}{
		{
			name:   "Single value",
			values: []string{".*"},
			res:    "match!(.parsed_data.test, r'.*')",
		},
		{
			name:   "Two values",
			values: []string{".*", ".+"},
			res:    "match!(.parsed_data.test, r'.*') || match!(.parsed_data.test, r'.+')",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := FilterRegexRule.Render(Args{"filter": map[string]interface{}{
				"Field": "parsed_data.test", "Values": tc.values,
			}})
			require.NoError(t, err)
			require.Equal(t, tc.res, res)
		})
	}
}

func TestFilterNotRegexRule(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		res    string
	}{
		{
			name:   "Single value",
			values: []string{".*"},
			res: strings.TrimSpace(`
if exists(.parsed_data.test) && is_string(.parsed_data.test) {
    matched = false
    matched0, err = match(.parsed_data.test, r'.*')
    if err != null {
        true
    }
    matched = matched || matched0
    !matched
} else {
    true
}
`),
		},
		{
			name:   "Two values",
			values: []string{".*", ".+"},
			res: strings.TrimSpace(`
if exists(.parsed_data.test) && is_string(.parsed_data.test) {
    matched = false
    matched0, err = match(.parsed_data.test, r'.*')
    if err != null {
        true
    }
    matched = matched || matched0
    matched1, err = match(.parsed_data.test, r'.+')
    if err != null {
        true
    }
    matched = matched || matched1
    !matched
} else {
    true
}
`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := FilterNotRegexRule.Render(Args{"filter": map[string]interface{}{
				"Field": "parsed_data.test", "Values": tc.values,
			}})
			require.NoError(t, err)
			require.Equal(t, tc.res, res)
		})
	}
}
