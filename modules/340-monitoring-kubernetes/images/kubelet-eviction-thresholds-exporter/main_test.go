package main

import "testing"

func Test_extractPercent(t *testing.T) {
	_ = map[string]string{
		"imagefs.available":  "10%",
		"imagefs.inodesFree": "10%",
		"nodefs.available":   "500Mi",
		"nodefs.inodesFree":  "500",
	}

	type args struct {
		realResource uint64
		signal       string
		evictionMap  map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"ten percent",
			args{
				realResource: 4000,
				signal:       "imagefs.available",
				evictionMap: map[string]string{
					"imagefs.available": "10%",
				},
			},
			"10",
			false,
		},
		{
			"bytes",
			args{
				realResource: 107374182400, // 100 GiB
				signal:       "imagefs.available",
				evictionMap: map[string]string{
					"imagefs.available": "10Gi",
				},
			},
			"10.00",
			false,
		},
		{
			"inodes",
			args{
				realResource: 100,
				signal:       "imagefs.inodes",
				evictionMap: map[string]string{
					"imagefs.inodes": "10",
				},
			},
			"10.00",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractPercent(tt.args.realResource, tt.args.signal, tt.args.evictionMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPercent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractPercent() got = %v, want %v", got, tt.want)
			}
		})
	}
}
