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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"github.com/gogo/protobuf/types"
	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	preAuthRLStage  = 0
	postAuthRLStage = 1

	APIFinalizerName = "kuadrant.io/envoyfilters"
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
		if apierrors.IsNotFound(err) {
			logger.Info("no API resource found.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get API object")
		return ctrl.Result{}, err
	}

	if api.GetDeletionTimestamp() != nil && controllerutil.ContainsFinalizer(&api, APIFinalizerName) {
		controllerutil.RemoveFinalizer(&api, APIFinalizerName)
		if err := r.BaseReconciler.UpdateResource(ctx, &api); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&api, APIFinalizerName) {
		controllerutil.AddFinalizer(&api, APIFinalizerName)
		if err := r.BaseReconciler.UpdateResource(ctx, &api); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	// get the list of matching virtualservices
	vslist := &istio.VirtualServiceList{}
	if len(api.Spec.VirtualServiceSelector) != 0 {
		matchingLabels := client.MatchingLabels{}
		matchingLabels = api.Spec.VirtualServiceSelector // interface requirements

		if err := r.Client().List(ctx, vslist, matchingLabels); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("no matching virtualservice resource found")
				return ctrl.Result{}, nil
			}
			logger.Error(err, "failed to fetch virtualservice list")
			return ctrl.Result{}, err
		}
	}
	logger.Info("number of virtualservices matched", "count", len(vslist.Items))

	rlpList := &apimv1alpha1.RateLimitPolicyList{}
	if len(api.Spec.RateLimitSelector) != 0 {
		matchingLabels := client.MatchingLabels{}
		matchingLabels = api.Spec.RateLimitSelector // interface requirements

		if err := r.Client().List(ctx, rlpList, matchingLabels); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "no ratelimitpolicy resource found")
				return ctrl.Result{}, nil
			}
			logger.Error(err, "failed to fetch ratelimitpolicy list")
			return ctrl.Result{}, err
		}
	}
	logger.Info("number of ratelimitpolicies matched", "count", len(rlpList.Items))

	for _, vs := range vslist.Items {
		vsNamespace := vs.Namespace
		for _, gateway := range vs.Spec.Gateways {
			objectKey := gatewayNameToObjectKey(gateway, vsNamespace)
			rateLimitPatch := rateLimitInitialPatch(objectKey)

			if err := controllerutil.SetOwnerReference(&api, rateLimitPatch, r.Client().Scheme()); err != nil {
				logger.Error(err, "failed to add owner ref to RateLimit filters patch")
				return ctrl.Result{}, err
			}
			if err := r.Client().Create(ctx, rateLimitPatch); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					logger.Error(err, "failed to patch-in RateLimit filters")
					return ctrl.Result{}, err
				}
			}
			logger.Info("created RateLimit filters patch")

			for _, host := range vs.Spec.Hosts {
				for _, rlp := range rlpList.Items {
					for _, route := range rlp.Spec.Routes {
						isRouteMatch := doesRouteMatch(vs.Spec.Http, route.Name)
						if !isRouteMatch {
							logger.Info("no match found for route %s", route.Name)
							continue
						}

						vHostName := host + ":80"
						var routePatchs []*istio.EnvoyFilter
						if route.Stage == apimv1alpha1.RateLimitStage_BOTH {
							routePatchs = append(routePatchs, routeRateLimitsPatch(vHostName, route.Name, route.Actions, apimv1alpha1.RateLimitStage_PREAUTH))
							routePatchs = append(routePatchs, routeRateLimitsPatch(vHostName, route.Name, route.Actions, apimv1alpha1.RateLimitStage_POSTAUTH))
						} else {
							routePatchs = append(routePatchs, routeRateLimitsPatch(vHostName, route.Name, route.Actions, route.Stage))
						}

						for _, routePatch := range routePatchs {
							if err := controllerutil.SetOwnerReference(&api, routePatch, r.Client().Scheme()); err != nil {
								logger.Error(err, "failed to add owner ref to route-level ratelimits patch")
								return ctrl.Result{}, err
							}
							if err := r.Client().Create(ctx, routePatch); err != nil {
								if !apierrors.IsAlreadyExists(err) {
									logger.Error(err, "failed to create route specific rate limit patch %s", routePatch.Name)
									return ctrl.Result{}, err
								}
							}
							logger.Info("created route level ratelimits patch", "name", routePatch.Name)
						}
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func doesRouteMatch(routeList []*networkingv1alpha3.HTTPRoute, targetRouteName string) bool {
	for _, route := range routeList {
		for _, match := range route.Match {
			if match.Name == targetRouteName {
				return true
			}
		}
	}
	return false
}

// gatewayNameToObjectKey converts <namespace/name> format string to k8 object key
func gatewayNameToObjectKey(gatewayName, defaultNamespace string) client.ObjectKey {
	split := strings.Split(gatewayName, "/")
	if len(split) == 2 {
		return client.ObjectKey{Name: split[1], Namespace: split[0]}
	}
	return client.ObjectKey{Namespace: defaultNamespace, Name: split[0]}
}

// rateLimitInitialPatch returns EnvoyFilter resource that patches-in
// - Add Preauth RateLimit Filter
// - Add PostAuth RateLimit Filter
// - Change cluster name pointing to Limitador in kuadrant-system namespace
// Note please make sure this patch is only created once per gateway.
func rateLimitInitialPatch(gateway client.ObjectKey) *istio.EnvoyFilter {

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
	postPatch := prePatch.DeepCopy()
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

	preAuthFilterPatch := networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
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

	postAuthFilterPatch := preAuthFilterPatch
	postAuthFilterPatch.Patch = postPatch

	clusterPatch := networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: networkingv1alpha3.EnvoyFilter_CLUSTER,
		Match: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: networkingv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Cluster{
				Cluster: &networkingv1alpha3.EnvoyFilter_ClusterMatch{
					Name: "outbound|8081||limitador.kuadrant-system.svc.cluster.local",
				},
			},
		},
		Patch: &networkingv1alpha3.EnvoyFilter_Patch{
			Operation: networkingv1alpha3.EnvoyFilter_Patch_MERGE,
			Value: &types.Struct{
				Fields: map[string]*types.Value{
					"name": {
						Kind: &types.Value_StringValue{
							StringValue: "rate-limit-cluster",
						},
					},
				},
			},
		},
	}

	factory := istioprovider.EnvoyFilterFactory{
		ObjectName: gateway.Name + "-ratelimit-filters",
		Namespace:  gateway.Namespace,
		Patches: []*networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
			// Ordering matters here!
			&preAuthFilterPatch,
			&postAuthFilterPatch,
			&clusterPatch,
		},
	}

	return factory.EnvoyFilter()
}

func routeRateLimitsPatch(vHostName, routeName string, actions []*apimv1alpha1.Action_Specifier, stage apimv1alpha1.RateLimit_Stage) *istio.EnvoyFilter {
	stage_value := apimv1alpha1.RateLimit_Stage_value[stage]
	patchUnstructured := map[string]interface{}{
		"operation": "MERGE",
		"value": map[string]interface{}{
			"route": map[string]interface{}{
				"rate_limits": []map[string]interface{}{
					{
						"stage":   stage_value,
						"actions": "ACTIONS", // this is replaced with actual value below
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

	// A nice trick to make it easier add-in actions
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
