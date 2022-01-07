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
	limitadorv1alpha1 "github.com/kuadrant/limitador-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type RL_GenericKey struct {
	DescriptorKey   string `json:"descriptor_key"`
	DescriptorValue string `json:"descriptor_value"`
}

type Action_Specifier struct {
	GenericKey RL_GenericKey `json:"generic_key"`
}

type Rule struct {
	URI    string `json:"uri"`
	Method string `json:"method"`
	// rule specific actions
	Actions []*Action_Specifier `json:"actions,omitempty"`
}

// RateLimitPolicySpec defines the desired state of RateLimitPolicy
type RateLimitPolicySpec struct {
	Phases []string `json:"phases"`
	Rules  []Rule   `json:"rules"`
	// these actions are used for all of the matching rules
	Actions []*Action_Specifier               `json:"actions,omitempty"`
	Limits  []limitadorv1alpha1.RateLimitSpec `json:"limits"`
}

//+kubebuilder:object:root=true

// RateLimitPolicy is the Schema for the ratelimitpolicies API
type RateLimitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RateLimitPolicySpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// RateLimitPolicyList contains a list of RateLimitPolicy
type RateLimitPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimitPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RateLimitPolicy{}, &RateLimitPolicyList{})
}
