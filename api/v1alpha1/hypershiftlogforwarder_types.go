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

// HyperShiftLogForwarderSpec defines the desired state of HyperShiftLogForwarder
type HyperShiftLogForwarderSpec struct {
	loggingv1.ClusterLogForwarderSpec `json:",inline"`
}

// HyperShiftLogForwarderStatus defines the observed state of HyperShiftLogForwarder
type HyperShiftLogForwarderStatus struct {
	loggingv1.ClusterLogForwarderStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HyperShiftLogForwarder is the Schema for the hypershiftlogforwarders API
type HyperShiftLogForwarder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HyperShiftLogForwarderSpec   `json:"spec,omitempty"`
	Status HyperShiftLogForwarderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HyperShiftLogForwarderList contains a list of HyperShiftLogForwarder
type HyperShiftLogForwarderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HyperShiftLogForwarder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HyperShiftLogForwarder{}, &HyperShiftLogForwarderList{})
}
