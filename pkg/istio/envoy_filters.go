package istio

import (
	"encoding/json"

	istioapiv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kuadrant/kuadrant-controller/pkg/common"
)

type HTTPFilterStage uint32

const (
	PreAuthStage HTTPFilterStage = iota
	PostAuthStage

	PatchedLimitadorClusterName = "rate-limit-cluster"
	PatchedWasmClusterName      = "remote-wasm-cluster"
)

type EnvoyFilterFactory struct {
	ObjectName string
	Namespace  string
	Patches    []*istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch
	Labels     map[string]string
}

func (v *EnvoyFilterFactory) EnvoyFilter() *istionetworkingv1alpha3.EnvoyFilter {
	return &istionetworkingv1alpha3.EnvoyFilter{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EnvoyFilter",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      v.ObjectName,
			Namespace: v.Namespace,
		},
		Spec: istioapiv1alpha3.EnvoyFilter{
			WorkloadSelector: &istioapiv1alpha3.WorkloadSelector{
				Labels: v.Labels,
			},
			ConfigPatches: v.Patches,
		},
	}
}

func LimitadorClusterEnvoyPatch() *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	// The patch defines the rate_limit_cluster, which provides the endpoint location of the external rate limit service.

	patchUnstructured := map[string]interface{}{
		"operation": "ADD",
		"value": map[string]interface{}{
			"name":                   PatchedLimitadorClusterName,
			"type":                   "STRICT_DNS",
			"connect_timeout":        "1s",
			"lb_policy":              "ROUND_ROBIN",
			"http2_protocol_options": map[string]interface{}{},
			"load_assignment": map[string]interface{}{
				"cluster_name": PatchedLimitadorClusterName,
				"endpoints": []map[string]interface{}{
					{
						"lb_endpoints": []map[string]interface{}{
							{
								"endpoint": map[string]interface{}{
									"address": map[string]interface{}{
										"socket_address": map[string]interface{}{
											"address":    common.LimitadorServiceClusterHost,
											"port_value": common.LimitadorServiceGrpcPort,
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
					Service: common.LimitadorServiceClusterHost,
				},
			},
		},
		Patch: patch,
	}
}

func WasmClusterEnvoyPatch() *istioapiv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch {
	patchUnstructured := map[string]interface{}{
		"operation": "ADD",
		"value": map[string]interface{}{
			"name":            PatchedWasmClusterName,
			"type":            "STRICT_DNS",
			"connect_timeout": "1s",
			"load_assignment": map[string]interface{}{
				"cluster_name": PatchedWasmClusterName,
				"endpoints": []map[string]interface{}{
					{
						"lb_endpoints": []map[string]interface{}{
							{
								"endpoint": map[string]interface{}{
									"address": map[string]interface{}{
										"socket_address": map[string]interface{}{
											"address":    "raw.githubusercontent.com",
											"port_value": 443,
										},
									},
								},
							},
						},
					},
				},
			},
			"dns_lookup_family": "V4_ONLY",
			"transport_socket": map[string]interface{}{
				"name": "envoy.transport_sockets.tls",
				"typed_config": map[string]interface{}{
					"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
					"sni":   "raw.githubusercontent.com",
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
			Context: istioapiv1alpha3.EnvoyFilter_GATEWAY,
		},
		Patch: patch,
	}
}
