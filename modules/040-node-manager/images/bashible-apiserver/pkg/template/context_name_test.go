package template

import (
	"testing"
)

func TestGetNodegroupContextKey(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{"valid", "ubuntu-lts.master-flomaster", "bundle-ubuntu-lts-master-flomaster", false},
		{"invalid", "ubuntu-lts-master-flomaster", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNodegroupContextKey(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodegroupContextKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetNodegroupContextKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetVersionContextKey(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{"valid", "ubuntu-lts.1-19", "bundle-ubuntu-lts-1.19", false},
		{"invalid", "ubuntu-lts-1-19", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetVersionContextKey(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVersionContextKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetVersionContextKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
