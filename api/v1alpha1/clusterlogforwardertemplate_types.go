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
	loggingv1 "github.com/openshift/cluster-logging-operator/apis/logging/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterLogForwarderTemplateSpec defines the desired state of ClusterLogForwarderTemplate
type ClusterLogForwarderTemplateSpec struct {
	Template loggingv1.ClusterLogForwarderSpec `json:"template"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterLogForwarderTemplate is the Schema for the clusterlogforwardertemplates API
type ClusterLogForwarderTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterLogForwarderTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterLogForwarderTemplateList contains a list of ClusterLogForwarderTemplate
type ClusterLogForwarderTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterLogForwarderTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterLogForwarderTemplate{}, &ClusterLogForwarderTemplateList{})
}
