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

package apim

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/gogo/protobuf/types"
	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

const (
	preAuthRLStage  = 0
	postAuthRLStage = 1
)

// APIReconciler reconciles a API object
type APIReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=apis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=apis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=apis/finalizers,verbs=update

func (r *APIReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("API", req.NamespacedName)
	logger.Info("Reconciling API object")
	ctx := logr.NewContext(eventCtx, logger)

	var api apimv1alpha1.API
	if err := r.Client().Get(ctx, req.NamespacedName, &api); err != nil {
		logger.Error(err, "failed to get API object")
		return ctrl.Result{}, err
	}

	virtualService := createVirtualService(&api)
	if err := r.Client().Create(ctx, virtualService); err != nil {
		logger.Error(err, "failed to create virtual service")
		return ctrl.Result{}, err
	}

	rateLimitPatch := rateLimitFiltersPatch(client.ObjectKey{
		Name:      "kuadrant-gateway",
		Namespace: "kuadrant-system",
	})
	if err := r.Client().Create(ctx, rateLimitPatch); err != nil {
		logger.Error(err, "failed to add rate limit filters")
		return ctrl.Result{}, err
	}

	if api.Spec.RateLimitSelector != nil {
		rlpList := &apimv1alpha1.RateLimitPolicyList{}
		matchingLabels := client.MatchingLabels{}
		matchingLabels = *api.Spec.RateLimitSelector // interface requirements
		if err := r.Client().List(ctx, rlpList, matchingLabels); err != nil {
			logger.Error(err, "failed to fetch ratelimitpolicy list")
		}

		for _, rlp := range rlpList.Items {
			for _, domain := range api.Spec.Domains {
				// Only two possiblity of phases: {Pre,Post}Auth
				for _, phase := range rlp.Spec.Phases {
					rateLimitStage := 0
					if phase == "postAuth" {
						rateLimitStage = 1
					}

					for _, rule := range rlp.Spec.Rules {
						routeName := rule.Method + rule.URI
						vHostName := domain + ":80" // Not sure if port is listener's or virtualservice destination

						if rule.Actions == nil {
							continue
						}
						routePatch := routeRateLimitsPatch(vHostName, routeName, rule.Actions, rateLimitStage)
						if err := r.Client().Create(ctx, routePatch); err != nil {
							logger.Error(err, "failed to create route specific rate limit patch")
							return ctrl.Result{}, err
						}
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// createVirtualService creates a virtualservice from Kuadrant's API resource
func createVirtualService(api *apimv1alpha1.API) *istio.VirtualService {
	httpRoutes := []*networkingv1alpha3.HTTPRoute{}
	httpMatchRequests := []*networkingv1alpha3.HTTPMatchRequest{}
	for path, operation := range api.Spec.Paths {
		method := "GET"
		if operation.Post != nil {
			method = "POST"
		}
		// name is used for selecting ratelimited routes.
		matchRequestName := method + path
		httpMatchRequest := &networkingv1alpha3.HTTPMatchRequest{
			Name: matchRequestName,
			Uri: &networkingv1alpha3.StringMatch{
				MatchType: &networkingv1alpha3.StringMatch_Exact{
					Exact: path,
				},
			},
			Method: &networkingv1alpha3.StringMatch{
				MatchType: &networkingv1alpha3.StringMatch_Exact{
					Exact: method,
				},
			},
		}
		httpMatchRequests = append(httpMatchRequests, httpMatchRequest)
	}

	// right now only allowing one destination.
	httpRoute := &networkingv1alpha3.HTTPRoute{
		Match: httpMatchRequests,
		Route: []*networkingv1alpha3.HTTPRouteDestination{
			{
				Destination: &networkingv1alpha3.Destination{
					Host: api.Spec.Upstreams[0].ServiceName,
					Port: &networkingv1alpha3.PortSelector{
						Number: api.Spec.Upstreams[0].Port,
					},
				},
			},
		},
	}
	httpRoutes = append(httpRoutes, httpRoute)

	factory := istioprovider.VirtualServiceFactory{
		ObjectName: api.Name + "-" + api.Namespace,
		Namespace:  api.Namespace,
		Hosts:      api.Spec.Domains,
		HTTPRoutes: httpRoutes,
		Gateways:   api.Spec.Gateways,
	}

	return factory.VirtualService()
}

// rateLimitEnvoyFilters returns EnvoyFilter that patches pre and post auth
// filters to the given gateway object.
func rateLimitFiltersPatch(gateway client.ObjectKey) *istio.EnvoyFilter {

	patchUnstructured := map[string]interface{}{
		"operation": "INSERT_BEFORE",
		"value": map[string]interface{}{
			"name": "envoy.filters.http.preauth.ratelimit",
			"typed_config": map[string]interface{}{
				"@type":             "type.googleapis.com/envoy.extensions.filters.http.ratelimit.v3.RateLimit",
				"domain":            "preauth",
				"stage":             preAuthRLStage,
				"failure_mode_deny": true,
				// If not specified, returns success immediately (can be useful for us)
				"rate_limit_service": map[string]interface{}{
					"transport_api_version": "V3",
					"grpc_service": map[string]interface{}{
						"timeout": "3s",
						"envoy_grpc": map[string]string{
							// It should be (outbound|8081||limitador.kuadrant-system.svc.cluster.local) but
							// limitador rejects request if '|' is present in the cluster name.
							"cluster_name": "rate-limit-cluster",
						},
					},
				},
			},
		},
	}

	patchRaw, _ := json.Marshal(patchUnstructured)

	prePatch := networkingv1alpha3.EnvoyFilter_Patch{}
	err := prePatch.UnmarshalJSON(patchRaw)
	if err != nil {
		panic(err)
	}
	postPatch := prePatch
	postPatch.Value.Fields["name"] = &types.Value{
		Kind: &types.Value_StringValue{
			StringValue: "envoy.filters.http.postauth.ratelimit",
		},
	}
	// update domain
	postPatch.Value.Fields["typed_config"].GetStructValue().Fields["domain"] = &types.Value{
		Kind: &types.Value_StringValue{
			StringValue: "postauth",
		},
	}
	// update stage
	postPatch.Value.Fields["typed_config"].GetStructValue().Fields["stage"] = &types.Value{
		Kind: &types.Value_NumberValue{
			NumberValue: postAuthRLStage,
		},
	}

	preEnvoyFilterPatch := networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: networkingv1alpha3.EnvoyFilter_HTTP_FILTER,
		Match: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: networkingv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Listener{
				Listener: &networkingv1alpha3.EnvoyFilter_ListenerMatch{
					PortNumber: 8080, // Kuadrant-gateway listens on this port by default
					FilterChain: &networkingv1alpha3.EnvoyFilter_ListenerMatch_FilterChainMatch{
						Filter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_FilterMatch{
							Name: "envoy.filters.network.http_connection_manager",
							SubFilter: &networkingv1alpha3.EnvoyFilter_ListenerMatch_SubFilterMatch{
								Name: "envoy.filters.http.router",
							},
						},
					},
				},
			},
		},
		Patch: &prePatch,
	}

	postEnvoyFilterPatch := preEnvoyFilterPatch
	postEnvoyFilterPatch.Patch = &postPatch

	factory := istioprovider.EnvoyFilterFactory{
		ObjectName: gateway.Name + "-ratelimit-filters",
		Namespace:  gateway.Namespace,
		Patches: []*networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			// Ordering matters here!
			&preEnvoyFilterPatch,
			&postEnvoyFilterPatch,
		},
	}

	return factory.EnvoyFilter()
}

func routeRateLimitsPatch(vHostName, routeName string, actions *[]apimv1alpha1.Action_Specifier, stage int) *istio.EnvoyFilter {
	patchUnstructured := map[string]interface{}{
		"operation": "MERGE",
		"value": map[string]interface{}{
			"route": map[string]interface{}{
				"rate_limits": []map[string]interface{}{
					{
						"stage":   stage,
						"actions": "ACTIONS",
					},
				},
			},
		},
	}
	patchRaw, _ := json.Marshal(patchUnstructured)

	jsonActions, err := json.Marshal(actions)
	if err != nil {
		// TODO: log this or just make it better
		return nil
	}

	// A nice trick to make things easy.
	stringPatch := string(patchRaw)
	stringPatch = strings.Replace(stringPatch, "\"ACTIONS\"", string(jsonActions), 1)
	fmt.Printf("STRING PATCH: %v", stringPatch)
	patchRaw = []byte(stringPatch)

	Patch := networkingv1alpha3.EnvoyFilter_Patch{}
	err = Patch.UnmarshalJSON(patchRaw)
	if err != nil {
		panic(err)
	}

	envoyFilterPatch := &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: networkingv1alpha3.EnvoyFilter_HTTP_ROUTE,
		Match: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: networkingv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_RouteConfiguration{
				RouteConfiguration: &networkingv1alpha3.EnvoyFilter_RouteConfigurationMatch{
					Vhost: &networkingv1alpha3.EnvoyFilter_RouteConfigurationMatch_VirtualHostMatch{
						Name: vHostName,
						Route: &networkingv1alpha3.EnvoyFilter_RouteConfigurationMatch_RouteMatch{
							Name: "." + routeName, // Istio adds '.' infront of names
						},
					},
				},
			},
		},
		Patch: &Patch,
	}

	// Need to make this unique among all the patches even if multi vhost with same
	// route name is present.
	objectName := "rate-limits-" + strings.ToLower(strings.Replace(routeName, "/", "-", -1))
	factory := istioprovider.EnvoyFilterFactory{
		ObjectName: objectName,
		Namespace:  "kuadrant-system",
		Patches: []*networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			envoyFilterPatch,
		},
	}

	return factory.EnvoyFilter()
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.API{}).
		Complete(r)
}
