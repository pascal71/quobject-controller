// +kubebuilder:object:generate=true
// +groupName=quobject.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type QuObjectBucketClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuObjectBucketClaimSpec   `json:"spec,omitempty"`
	Status QuObjectBucketClaimStatus `json:"status,omitempty"`
}

type QuObjectBucketClaimSpec struct {
	BucketName         string            `json:"bucketName,omitempty"`
	GenerateBucketName string            `json:"generateBucketName,omitempty"`
	StorageClassName   string            `json:"storageClassName,omitempty"`
	AdditionalConfig   map[string]string `json:"additionalConfig,omitempty"`
}

type QuObjectBucketClaimStatus struct {
	Phase        string `json:"phase,omitempty"`
	BucketName   string `json:"bucketName,omitempty"`
	SecretRef    string `json:"secretRef,omitempty"`
	ConfigMapRef string `json:"configMapRef,omitempty"`
}

// +kubebuilder:object:root=true
type QuObjectBucketClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuObjectBucketClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuObjectBucketClaim{}, &QuObjectBucketClaimList{})
}
