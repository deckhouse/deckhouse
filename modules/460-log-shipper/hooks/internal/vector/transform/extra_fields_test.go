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

package transform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_processExtraFieldKey(t *testing.T) {
	type args struct {
		key   string
		value string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple::template::value",
			args: args{key: "key1", value: "{{ simple_template }}"},
			want: " if exists(.parsed_data.simple_template) { .key1=.parsed_data.simple_template } \n",
		},
		{
			name: "template::value::with::hypens",
			args: args{key: "key2", value: "{{ X-Check-hypen }}"},
			want: " if exists(.parsed_data.\"X-Check-hypen\") { .key2=.parsed_data.\"X-Check-hypen\" } \n",
		},
		{
			name: "value::with::dots",
			args: args{key: "key3", value: "dot.ted.value"},
			want: " .key3=\"dot.ted.value\" \n",
		},
		{
			name: "template::value::with::dots",
			args: args{key: "key4", value: "{{ ve.ry.dot.ted.va.lue }}"},
			want: " if exists(.parsed_data.ve.ry.dot.ted.va.lue) { .key4=.parsed_data.ve.ry.dot.ted.va.lue } \n",
		},
		{
			name: "template::value::with::dots::and::hypens::escaped",
			args: args{key: "key5", value: `{{ va\.lue-hy\.pen }}`},
			want: " if exists(.parsed_data.\"va.lue-hy.pen\") { .key5=.parsed_data.\"va.lue-hy.pen\" } \n",
		},
		{
			name: "template::empty::value",
			args: args{key: "key6", value: "{{  }}"},
			want: " .key6=\"{{  }}\" \n",
		},
		{
			name: "empty::value",
			args: args{key: "key7", value: ""},
			want: " .key7=\"\" \n",
		},
		{
			name: "template::value::parsed_data",
			args: args{key: "key8", value: "{{ parsed_data }}"},
			want: " if exists(.parsed_data) { .key8=.parsed_data } \n",
		},
		{
			name: "key::with::hypens::and::dots",
			args: args{key: "key9.with-dots.and-hypens", value: "{{ parsed_data }}"},
			want: " if exists(.parsed_data) { .\"key9.with-dots.and-hypens\"=.parsed_data } \n",
		},
		{
			name: "empty::key::and::value",
			args: args{key: "", value: ""},
			want: "",
		},
		{
			name: "template::value::with::parsed_data",
			args: args{key: "key11", value: "{{ parsed_data.check.parsed-data }}"},
			want: " if exists(.parsed_data.check.\"parsed-data\") { .key11=.parsed_data.check.\"parsed-data\" } \n",
		},
		{
			name: "template::value::with::dots::escapes::hypens::and::indexing",
			args: args{key: "key12", value: `{{ pay\.lo[3].te\.st }}`},
			want: " if exists(.parsed_data.\"pay.lo\"[3].\"te.st\") { .key12=.parsed_data.\"pay.lo\"[3].\"te.st\" } \n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, processExtraFieldKey(tt.args.key, tt.args.value))
		})
	}
}
