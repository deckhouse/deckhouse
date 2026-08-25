package proxy

import (
	"github.com/distribution/reference"
	"testing"
)

func TestGetRemoteRepositoryName(t *testing.T) {
	tests := []struct {
		name             string
		remotePathOnly   string
		localPathAlias   string
		repoName         string
		expectedRepoName string
		expectError      bool
	}{
		{
			name:             "No alias, no path change",
			remotePathOnly:   "",
			localPathAlias:   "",
			repoName:         "myrepo",
			expectedRepoName: "myrepo",
			expectError:      false,
		},
		{
			name:             "Alias exists, path updated",
			remotePathOnly:   "remote",
			localPathAlias:   "local",
			repoName:         "local/repo",
			expectedRepoName: "remote/repo",
			expectError:      false,
		},
		{
			name:             "Alias exists but no match",
			remotePathOnly:   "remote",
			localPathAlias:   "local",
			repoName:         "other/repo",
			expectedRepoName: "other/repo",
			expectError:      false,
		},
		{
			name:             "Alias without remotePathOnly",
			remotePathOnly:   "",
			localPathAlias:   "local",
			repoName:         "local/repo",
			expectedRepoName: "",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &proxyingRegistry{
				remotePathOnly: tt.remotePathOnly,
				localPathAlias: tt.localPathAlias,
			}

			name, _ := reference.WithName(tt.repoName)
			result, err := pr.getRemoteRepositoryName(name)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("did not expect error but got %v", err)
				}
				if result.String() != tt.expectedRepoName {
					t.Errorf("expected repo name %s, but got %s", tt.expectedRepoName, result.String())
				}
			}
		})
	}
}

func TestRepositoryIsAllowed(t *testing.T) {
	tests := []struct {
		name            string
		remotePathOnly  string
		localPathAlias  string
		repoName        string
		expectedAllowed bool
		expectError     bool
	}{
		{
			name:            "Allowed repository without remotePathOnly option",
			remotePathOnly:  "",
			localPathAlias:  "",
			repoName:        "remote/repo",
			expectedAllowed: true,
			expectError:     false,
		},
		{
			name:            "Allowed repository without alias",
			remotePathOnly:  "remote",
			localPathAlias:  "",
			repoName:        "remote/repo",
			expectedAllowed: true,
			expectError:     false,
		},
		{
			name:            "Allowed repository with alias",
			remotePathOnly:  "remote",
			localPathAlias:  "local",
			repoName:        "local/repo",
			expectedAllowed: true,
			expectError:     false,
		},
		{
			name:            "Not allowed repository",
			remotePathOnly:  "remote",
			localPathAlias:  "",
			repoName:        "other/repo",
			expectedAllowed: false,
			expectError:     true,
		},
		{
			name:            "Not allowed with alias",
			remotePathOnly:  "remote",
			localPathAlias:  "local",
			repoName:        "other/repo",
			expectedAllowed: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &proxyingRegistry{
				remotePathOnly: tt.remotePathOnly,
				localPathAlias: tt.localPathAlias,
			}

			name, _ := reference.WithName(tt.repoName)
			result, err := pr.repositoryIsAllowed(name)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if result {
					t.Errorf("expected false, but got true")
				}
			} else {
				if err != nil {
					t.Errorf("did not expect error but got %v", err)
				}
				if result != tt.expectedAllowed {
					t.Errorf("expected %v, but got %v", tt.expectedAllowed, result)
				}
			}
		})
	}
}
