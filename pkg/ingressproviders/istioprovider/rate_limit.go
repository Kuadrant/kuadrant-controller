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

package istioprovider

import (
	"context"
	"fmt"
	"reflect"

	istioapiv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/common"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

func (is *IstioProvider) reconcileRateLimit(ctx context.Context, apip *networkingv1beta1.APIProduct) error {
	desiredEnvoyFilter := rateLimitEnvoyFilter(apip)

	if apip.Spec.RateLimit == nil {
		common.TagObjectToDelete(desiredEnvoyFilter)
	}

	err := is.ReconcileIstioEnvoyFilter(ctx, desiredEnvoyFilter, envoyFilterBasicMutator)
	if err != nil {
		return err
	}

	return nil
}

func rateLimitEnvoyFilter(apip *networkingv1beta1.APIProduct) *istionetworkingv1alpha3.EnvoyFilter {
	envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EnvoyFilter",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kuadrant-%s.%s-ratelimit", apip.Name, apip.Namespace),
			Namespace: common.KuadrantNamespace,
		},
		Spec: istioapiv1alpha3.EnvoyFilter{
			WorkloadSelector: &istioapiv1alpha3.WorkloadSelector{
				Labels: map[string]string{"istio": "kuadrant-system"},
			},
			ConfigPatches: []*istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				httpFilterEnvoyPatch(apip),
				clusterEnvoyPatch(),
			},
		},
	}

	for _, host := range apip.Spec.Routing.Hosts {
		envoyFilter.Spec.ConfigPatches = append(envoyFilter.Spec.ConfigPatches, rateLimitActionsEnvoyPatch(host))
	}

	return envoyFilter
}

func rateLimitActionsEnvoyPatch(host string) *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	// defines the route configuration on which to rate limit
	patch := &istioapiv1alpha3.EnvoyFilter_Patch{}

	patchRaw := []byte(
		`{
			"operation": "MERGE",
			"value": {
				"rate_limits": [
					{
						"actions": [
							{
								"remote_address": {}
							}
						]
					}
				]
			}
		}`)

	err := patch.UnmarshalJSON(patchRaw)
	if err != nil {
		panic(err)
	}

	return &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istioapiv1alpha3.EnvoyFilter_VIRTUAL_HOST,
		Match: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: istioapiv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_RouteConfiguration{
				RouteConfiguration: &istioapiv1alpha3.EnvoyFilter_RouteConfigurationMatch{
					Vhost: &istioapiv1alpha3.EnvoyFilter_RouteConfigurationMatch_VirtualHostMatch{
						Name: fmt.Sprintf("%s:80", host),
						Route: &istioapiv1alpha3.EnvoyFilter_RouteConfigurationMatch_RouteMatch{
							Action: istioapiv1alpha3.EnvoyFilter_RouteConfigurationMatch_RouteMatch_ANY,
						},
					},
				},
			},
		},
		Patch: patch,
	}
}

func httpFilterEnvoyPatch(apip *networkingv1beta1.APIProduct) *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	// The patch inserts the envoy.filters.http.ratelimit global envoy filter
	// filter into the HTTP_FILTER chain.
	// The rate_limit_service field specifies the external rate limit service, rate_limit_cluster in this case.
	patchUnstructured := map[string]interface{}{
		"operation": "INSERT_BEFORE",
		"value": map[string]interface{}{
			"name": "envoy.filters.http.ratelimit",
			"typed_config": map[string]interface{}{
				"@type":             "type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit",
				"domain":            apip.RateLimitDomainName(),
				"failure_mode_deny": true,
				"rate_limit_service": map[string]interface{}{
					"transport_api_version": "V3",
					"grpc_service": map[string]interface{}{
						"timeout": "3s",
						"envoy_grpc": map[string]string{
							"cluster_name": "rate_limit_cluster",
						},
					},
				},
			},
		},
	}

	patchRaw, _ := json.Marshal(patchUnstructured)

	patch := &istioapiv1alpha3.EnvoyFilter_Patch{}
	err := patch.UnmarshalJSON(patchRaw)
	if err != nil {
		panic(err)
	}

	// insert global rate limit http filter before external AUTH
	return &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istioapiv1alpha3.EnvoyFilter_HTTP_FILTER,
		Match: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: istioapiv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
				Listener: &istioapiv1alpha3.EnvoyFilter_ListenerMatch{
					FilterChain: &istioapiv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
						Filter: &istioapiv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
							Name: "envoy.filters.network.http_connection_manager",
							SubFilter: &istioapiv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
								Name: "envoy.filters.http.ext_authz",
							},
						},
					},
				},
			},
		},
		Patch: patch,
	}
}

func clusterEnvoyPatch() *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	// The patch defines the rate_limit_cluster, which provides the endpoint location of the external rate limit service.

	patchUnstructured := map[string]interface{}{
		"operation": "ADD",
		"value": map[string]interface{}{
			"name":                   "rate_limit_cluster",
			"type":                   "STRICT_DNS",
			"connect_timeout":        "1s",
			"lb_policy":              "ROUND_ROBIN",
			"http2_protocol_options": map[string]interface{}{},
			"load_assignment": map[string]interface{}{
				"cluster_name": "rate_limit_cluster",
				"endpoints": []map[string]interface{}{
					{
						"lb_endpoints": []map[string]interface{}{
							{
								"endpoint": map[string]interface{}{
									"address": map[string]interface{}{
										"socket_address": map[string]interface{}{
											"address":    LimitadorServiceClusterHost,
											"port_value": LimitadorServiceGrpcPort,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	patchRaw, _ := json.Marshal(patchUnstructured)

	patch := &istioapiv1alpha3.EnvoyFilter_Patch{}
	err := patch.UnmarshalJSON(patchRaw)
	if err != nil {
		panic(err)
	}

	return &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: istioapiv1alpha3.EnvoyFilter_CLUSTER,
		Match: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			ObjectTypes: &istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Cluster{
				Cluster: &istioapiv1alpha3.EnvoyFilter_ClusterMatch{
					Service: LimitadorServiceClusterHost,
				},
			},
		},
		Patch: patch,
	}
}

// TODO(eastizle): EnvoyFilter does not exist in "istio.io/client-go/pkg/apis/networking/v1alpha3". Alternative in v1beta1???
func (is *IstioProvider) ReconcileIstioEnvoyFilter(ctx context.Context, desired *istionetworkingv1alpha3.EnvoyFilter, mutatefn reconcilers.MutateFn) error {
	return is.ReconcileResource(ctx, &istionetworkingv1alpha3.EnvoyFilter{}, desired, mutatefn)
}

func envoyFilterBasicMutator(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*istionetworkingv1alpha3.EnvoyFilter)
	if !ok {
		return false, fmt.Errorf("%T is not a *istionetworkingv1alpha3.EnvoyFilter", existingObj)
	}
	desired, ok := desiredObj.(*istionetworkingv1alpha3.EnvoyFilter)
	if !ok {
		return false, fmt.Errorf("%T is not a *istionetworkingv1alpha3.EnvoyFilter", desiredObj)
	}

	updated := false
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		updated = true
	}

	return updated, nil
}
