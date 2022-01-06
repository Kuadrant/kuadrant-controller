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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OPARef struct {
	URL       string                   `json:"URL,omitempty"`
	ConfigMap *v1.LocalObjectReference `json:"configMap,omitempty"`
}

type API_Metadata struct {
	Version     string `json:"version"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	OpenAPIRef  OPARef `json:"openAPIRef,omitempty"`
}

type Operation_GET struct{}

type Upstream struct {
	URL         string `json:"url,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Port        uint32 `json:"port,omitempty"`
	TLS         string `json:"tls,omitempty"`
}

type Operation_POST struct {
	Upstreams []Upstream `json:"upstreams"`
}

type Operation struct {
	Get  *Operation_GET  `json:"get,omitempty"`
	Post *Operation_POST `json:"post,omitempty"`
}

type APISpec struct {
	Domains            []string             `json:"domains"`
	Gateways           []string             `json:"gateways,omitempty"`
	Info               API_Metadata         `json:"info"`
	Upstreams          []Upstream           `json:"upstreams"`
	Paths              map[string]Operation `json:"paths"`
	AuthConfigSelector map[string]string    `json:"authConfigSelector,omitempty"`
	RateLimitSelector  *map[string]string   `json:"rateLimitSelector,omitempty"`
}

// +kubebuilder:object:root=true
// API is the Schema for the apis API
type API struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec APISpec `json:"spec"`
}

// +kubebuilder:object:root=true

// APIList contains a list of API
type APIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []API `json:"items"`
}

func init() {
	SchemeBuilder.Register(&API{}, &APIList{})
}
