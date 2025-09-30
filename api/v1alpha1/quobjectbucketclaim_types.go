// +kubebuilder:object:generate=true
// +groupName=quobject.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RetainPolicy defines what happens to the bucket when the claim is deleted
// +kubebuilder:validation:Enum=Retain;Delete
type RetainPolicy string

const (
	// RetainPolicyRetain keeps the bucket when the claim is deleted (default)
	RetainPolicyRetain RetainPolicy = "Retain"
	// RetainPolicyDelete deletes the bucket when the claim is deleted
	RetainPolicyDelete RetainPolicy = "Delete"
)

// QuObjectBucketClaimSpec defines the desired state of QuObjectBucketClaim
type QuObjectBucketClaimSpec struct {
	// BucketName is the explicit name for the bucket.
	// If specified, this exact name will be used.
	// +optional
	BucketName string `json:"bucketName,omitempty"`

	// GenerateBucketName is the prefix for generated bucket names.
	// If specified (and BucketName is not), a random suffix will be added.
	// +optional
	GenerateBucketName string `json:"generateBucketName,omitempty"`

	// StorageClassName specifies the storage class to use
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// RetainPolicy determines if the bucket should be retained or deleted
	// when the claim is deleted. Default is "Retain".
	// +kubebuilder:default=Retain
	// +optional
	RetainPolicy RetainPolicy `json:"retainPolicy,omitempty"`

	// AdditionalConfig contains additional configuration for the bucket
	// +optional
	AdditionalConfig map[string]string `json:"additionalConfig,omitempty"`
}

// QuObjectBucketClaimStatus defines the observed state of QuObjectBucketClaim
type QuObjectBucketClaimStatus struct {
	// Phase represents the current phase of the bucket claim
	// +optional
	Phase string `json:"phase,omitempty"`

	// BucketName is the actual name of the created bucket
	// +optional
	BucketName string `json:"bucketName,omitempty"`

	// SecretRef is the name of the secret containing bucket credentials
	// +optional
	SecretRef string `json:"secretRef,omitempty"`

	// ConfigMapRef is the name of the configmap containing bucket configuration
	// +optional
	ConfigMapRef string `json:"configMapRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="BucketName",type=string,JSONPath=`.status.bucketName`
// +kubebuilder:printcolumn:name="RetainPolicy",type=string,JSONPath=`.spec.retainPolicy`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QuObjectBucketClaim is the Schema for the quobjectbucketclaims API
type QuObjectBucketClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuObjectBucketClaimSpec   `json:"spec,omitempty"`
	Status QuObjectBucketClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QuObjectBucketClaimList contains a list of QuObjectBucketClaim
type QuObjectBucketClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuObjectBucketClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuObjectBucketClaim{}, &QuObjectBucketClaimList{})
}
