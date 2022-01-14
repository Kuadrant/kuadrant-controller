/*
Copyright 2021 Red Hat, Inc.

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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WASMFilterSpec defines the desired state of WASMFilter
type WASMFilterSpec struct {

	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Type is used to identify which wasmplugin to use
	Type string `json:"type,omitempty"`
	// HTTPRoute identifies which http route this config is for
	HTTPRouteRef HTTPRouteRef `json:"httpRouteRef"`

	// Conifg configures the plugin
	// +kubebuilder:pruning:PreserveUnknownFields
	PluginConfig runtime.RawExtension `json:"pluginConfig,omitempty"`
}

func (wf WASMFilter) DecodePluginConfig() (map[string]interface{}, error) {
	data := map[string]interface{}{}
	if err := json.Unmarshal(wf.Spec.PluginConfig.Raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

type HTTPRouteRef struct {
	Name     string   `json:"name,omitempty"`
	Backends []string `json:"backends,omitempty"`
}

// WASMFilterStatus defines the observed state of WASMFilter
type WASMFilterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WASMFilter is the Schema for the wasmfilters API
type WASMFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WASMFilterSpec   `json:"spec,omitempty"`
	Status WASMFilterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WASMFilterList contains a list of WASMFilter
type WASMFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WASMFilter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WASMFilter{}, &WASMFilterList{})
}
