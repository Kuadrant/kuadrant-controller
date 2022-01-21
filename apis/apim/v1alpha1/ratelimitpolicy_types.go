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

// +kubebuilder:validation:Enum=PREAUTH;POSTAUTH;BOTH
type RateLimit_Stage string

// +kubebuilder:validation:Enum=HTTPRoute;VirtualService
type NetworkingRef_Type string

const (
	RateLimitStage_PREAUTH  RateLimit_Stage = "PREAUTH"
	RateLimitStage_POSTAUTH RateLimit_Stage = "POSTAUTH"
	RateLimitStage_BOTH     RateLimit_Stage = "BOTH"

	NetworkingRefType_HR NetworkingRef_Type = "HTTPRoute"
	NetworkingRefType_VS NetworkingRef_Type = "VirtualService"
)

var RateLimit_Stage_name = map[int32]string{
	0: "PREAUTH",
	1: "POSTAUTH",
	2: "BOTH",
}

var RateLimit_Stage_value = map[RateLimit_Stage]int32{
	"PREAUTH":  0,
	"POSTAUTH": 1,
	"BOTH":     2,
}

type Route struct {
	// name of the route present in the virutalservice
	Name string `json:"name"`
	// Definfing phase at which rate limits will be applied.
	// Valid values are: PREAUTH, POSTAUTH, BOTH
	Stage RateLimit_Stage `json:"stage"`
	// rule specific actions
	Actions []*Action_Specifier `json:"actions,omitempty"`
}

type NetworkingRef struct {
	Type NetworkingRef_Type `json:"type"`
	Name string             `json:"name"`
}

// RateLimitPolicySpec defines the desired state of RateLimitPolicy
type RateLimitPolicySpec struct {
	NetworkingRef []NetworkingRef `json:"networkingRef,omitempty"`
	// route specific staging and actions
	//+listType=map
	//+listMapKey=name
	Routes []Route `json:"routes,omitempty"`
	// these actions are used for all of the matching rules
	Actions []*Action_Specifier               `json:"actions,omitempty"`
	Limits  []limitadorv1alpha1.RateLimitSpec `json:"limits,omitempty"`
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
