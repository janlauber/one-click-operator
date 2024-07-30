/*
Copyright 2023.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ImageSpec struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
}

type SecurityContextSpec struct {
	FsGroup                  int64            `json:"fsGroup,omitempty"`
	RunAsUser                int64            `json:"runAsUser,omitempty"`
	RunAsGroup               int64            `json:"runAsGroup,omitempty"`
	AllowPrivilegeEscalation bool             `json:"allowPrivilegeEscalation,omitempty"`
	RunAsNonRoot             bool             `json:"runAsNonRoot,omitempty"`
	ReadOnlyRootFilesystem   bool             `json:"readOnlyRootFilesystem,omitempty"`
	Privileged               bool             `json:"privileged,omitempty"`
	Capabilities             CapabilitiesSpec `json:"capabilities,omitempty"`
}

type CapabilitiesSpec struct {
	Add  []string `json:"add,omitempty"`
	Drop []string `json:"drop,omitempty"`
}

type HorizontalScaleSpec struct {
	MinReplicas                    int32 `json:"minReplicas"`
	MaxReplicas                    int32 `json:"maxReplicas"`
	TargetCPUUtilizationPercentage int32 `json:"targetCPUUtilizationPercentage"`
}

type ResourceRequirements struct {
	Requests ResourceList `json:"requests"`
	Limits   ResourceList `json:"limits"`
}

type ResourceList struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SecretItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type VolumeSpec struct {
	Name         string `json:"name"`
	MountPath    string `json:"mountPath"`
	Size         string `json:"size"`
	StorageClass string `json:"storageClass,omitempty"`
}

type InterfaceSpec struct {
	Name    string      `json:"name"`
	Port    int32       `json:"port"`
	Ingress IngressSpec `json:"ingress,omitempty"`
}

type IngressSpec struct {
	IngressClass string            `json:"ingressClass"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Rules        []IngressRule     `json:"rules"`
}

type IngressRule struct {
	Host          string `json:"host"`
	Path          string `json:"path"`
	TLS           bool   `json:"tls"`
	TlsSecretName string `json:"tlsSecretName,omitempty"`
}

type CronJobSpec struct {
	Name         string               `json:"name"`
	Suspend      bool                 `json:"suspend"`
	Image        ImageSpec            `json:"image"`
	Schedule     string               `json:"schedule"`
	Command      []string             `json:"command,omitempty"`
	Args         []string             `json:"args,omitempty"`
	MaxRetries   int32                `json:"maxRetries,omitempty"`
	BackoffLimit int32                `json:"backoffLimit,omitempty"`
	Env          []EnvVar             `json:"env,omitempty"`
	Resources    ResourceRequirements `json:"resources"`
}

// RolloutSpec defines the desired state of Rollout
type RolloutSpec struct {
	Args               []string             `json:"args,omitempty"`
	Command            []string             `json:"command,omitempty"`
	RolloutStrategy    string               `json:"rolloutStrategy,omitempty"`
	Image              ImageSpec            `json:"image"`
	SecurityContext    SecurityContextSpec  `json:"securityContext,omitempty"`
	HorizontalScale    HorizontalScaleSpec  `json:"horizontalScale"`
	Resources          ResourceRequirements `json:"resources"`
	Env                []EnvVar             `json:"env,omitempty"`
	Secrets            []SecretItem         `json:"secrets,omitempty"`
	Volumes            []VolumeSpec         `json:"volumes,omitempty"`
	Interfaces         []InterfaceSpec      `json:"interfaces,omitempty"`
	ServiceAccountName string               `json:"serviceAccountName"`
	CronJobs           []CronJobSpec        `json:"cronjobs,omitempty"`
	NodeSelector       map[string]string    `json:"nodeSelector,omitempty"`
	Tolerations        []corev1.Toleration  `json:"tolerations,omitempty"`
	HostAliases        []corev1.HostAlias   `json:"hostAliases,omitempty"`
}

type Resources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type DeploymentResources struct {
	RequestSum Resources `json:"requestSum"`
	LimitSum   Resources `json:"limitSum"`
}

type DeploymentStatus struct {
	Replicas  int32               `json:"replicas"`
	PodNames  []string            `json:"podNames"`
	Resources DeploymentResources `json:"resources"`
	Status    string              `json:"status"`
}

type ServiceStatus struct {
	Name   string  `json:"name"`
	Ports  []int32 `json:"ports"`
	Status string  `json:"status"`
}

type IngressStatus struct {
	Name   string   `json:"name"`
	Hosts  []string `json:"hosts"`
	Status string   `json:"status"`
}

type VolumeStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// RolloutStatus defines the observed state of Rollout
type RolloutStatus struct {
	Deployment DeploymentStatus `json:"deployment"`
	Services   []ServiceStatus  `json:"services,omitempty"`
	Ingresses  []IngressStatus  `json:"ingresses,omitempty"`
	Volumes    []VolumeStatus   `json:"volumes,omitempty"`
}

//+kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image.repository"
//+kubebuilder:printcolumn:name="ImageTag",type="string",JSONPath=".spec.image.tag"
//+kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.horizontalScale.minReplicas"
//+kubebuilder:printcolumn:name="Deployment Status",type="string",JSONPath=".status.deployment.status"
//+kubebuilder:printcolumn:name="Service Status",type="string",JSONPath=".status.services[*].status"
//+kubebuilder:printcolumn:name="Ingress Status",type="string",JSONPath=".status.ingresses[*].status"
//+kubebuilder:printcolumn:name="Volume Status",type="string",JSONPath=".status.volumes[*].status"
//+kubebuilder:printcolumn:name="Service Account",type="string",JSONPath=".spec.serviceAccountName"
//+kubebuilder:printcolumn:name="Rollout Strategy",type="string",JSONPath=".spec.rolloutStrategy"
//+kubebuilder:printcolumn:name="Creation Timestamp",type="date",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Rollout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RolloutSpec   `json:"spec,omitempty"`
	Status RolloutStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RolloutList contains a list of Rollout
type RolloutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rollout `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rollout{}, &RolloutList{})
}
