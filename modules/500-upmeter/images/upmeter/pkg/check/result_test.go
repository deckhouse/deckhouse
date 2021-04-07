package check

import "testing"

func TestProbeResult_Status(t *testing.T) {
	ref := ProbeRef{}

	tests := []struct {
		name  string
		added []Result
		want  Status
	}{
		{
			name: "nothing means nodata",
			want: nodata,
		}, {
			name:  "one Up means Up",
			added: []Result{NewResult(ref, "x", Up)},
			want:  Up,
		}, {
			name:  "one Down means Down",
			added: []Result{NewResult(ref, "x", Down)},
			want:  Down,
		}, {
			name:  "one Unknown means Unknown",
			added: []Result{NewResult(ref, "x", Unknown)},
			want:  Unknown,
		}, {
			name: "Up,Down means Down",
			added: []Result{
				NewResult(ref, "x", Up),
				NewResult(ref, "l", Down)},
			want: Down,
		}, {
			name: "Down,Up means Down",
			added: []Result{
				NewResult(ref, "x", Down),
				NewResult(ref, "l", Up)},
			want: Down,
		}, {
			name: "Down changed by Up means Up",
			added: []Result{
				NewResult(ref, "x", Down),
				NewResult(ref, "x", Up)},
			want: Up,
		}, {
			name: "Up changed by Down means Down",
			added: []Result{
				NewResult(ref, "x", Up),
				NewResult(ref, "x", Down)},
			want: Down,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			pr := NewProbeResult(ref)
			for _, res := range tt.added {
				pr.Add(res)
			}

			if got := pr.Status(); got != tt.want {
				t.Errorf("ProbeResult.Status() = %v, want %v", got, tt.want)
			}
		})
	}
}
