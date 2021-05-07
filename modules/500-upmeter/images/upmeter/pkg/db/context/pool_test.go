package context

import "testing"

func Test_buildUri(t *testing.T) {
	type args struct {
		path string
		opts map[string]string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "absolute path",
			args: args{path: "/db"},
			want: "/db",
		},
		{
			name: "relative path",
			args: args{path: "./db"},
			want: "./db",
		},
		{
			name: "file-relative path",
			args: args{path: "file://./db"},
			want: "file://./db",
		},
		{
			name: "path with options",
			args: args{
				path: "/db",
				opts: map[string]string{"a": "aaa", "x": "xxx"},
			},
			want: "/db?a=aaa&x=xxx",
		},
		{
			name: ":memory:",
			args: args{path: ":memory:"},
			want: ":memory:",
		},
		{
			name: ":memory: with options",
			args: args{
				path: ":memory:",
				opts: map[string]string{"a": "aaa", "x": "xxx"},
			},
			want: ":memory:?a=aaa&x=xxx",
		},
	}

	for _, tt := range tests {
		got := buildUri(tt.args.path, tt.args.opts)
		if got != tt.want {
			t.Errorf("uriby(): got=%s, want=%s", got, tt.want)
		}
	}
}
