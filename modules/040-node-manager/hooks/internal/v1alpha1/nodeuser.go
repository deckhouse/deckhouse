package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeUser is a linux user for all nodes.
type NodeUser struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines parameters for user.
	Spec NodeUserSpec `json:"spec"`
}

type NodeUserSpec struct {
	// Unique user ID.
	UID int32 `json:"uid"`

	// Ssh public key.
	SSHPublicKey string `json:"sshPublicKey"`

	// Hashed user password for /etc/shadow.
	PasswordHash string `json:"passwordHash"`

	// Is node user belongs to the sudo group.
	IsSudoer bool `json:"isSudoer"`

	// Additional system groups.
	ExtraGroups []string `json:"extraGroups,omitempty"`
}
