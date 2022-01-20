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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"github.com/gogo/protobuf/types"
	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"
	limitador "github.com/kuadrant/kuadrant-controller/pkg/ratelimitproviders/limitador"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	"github.com/kuadrant/limitador-operator/api/v1alpha1"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

const (
	finalizerName = "kuadrant.io/ratelimitpolicy"

	preAuthRLStage  = 0
	postAuthRLStage = 1

	RLPAnnotationVSName         = "kuadrant.io/virtualservice"
	VSAnnotationRateLimitPolicy = "kuadrant.io/ratelimitpolicy"

	DefaultLimitadorClusterName = "outbound|8081||limitador.kuadrant-system.svc.cluster.local"
	PatchedLimitadorClusterName = "rate-limit-cluster"
)

// RateLimitPolicyReconciler reconciles a RateLimitPolicy object
type RateLimitPolicyReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=ratelimitpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=ratelimitpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=ratelimitpolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RateLimitPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *RateLimitPolicyReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("RateLimitPolicy", req.NamespacedName)
	logger.Info("Reconciling RateLimitPolicy")
	ctx := logr.NewContext(eventCtx, logger)

	var rlp apimv1alpha1.RateLimitPolicy
	if err := r.Client().Get(ctx, req.NamespacedName, &rlp); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("no rate limit policy found.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get RateLimitPolicy")
		return ctrl.Result{}, err
	}

	if rlp.GetDeletionTimestamp() != nil && controllerutil.ContainsFinalizer(&rlp, finalizerName) {
		controllerutil.RemoveFinalizer(&rlp, finalizerName)
		if err := r.BaseReconciler.UpdateResource(ctx, &rlp); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&rlp, finalizerName) {
		controllerutil.AddFinalizer(&rlp, finalizerName)
		if err := r.BaseReconciler.UpdateResource(ctx, &rlp); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	if err := r.reconcileLimits(ctx, &rlp); err != nil {
		return ctrl.Result{}, err
	}

	vsName, ok := rlp.Annotations[RLPAnnotationVSName]
	if !ok {
		logger.Info("virtualservice annotation on ratelimitpolicy not found")
		return ctrl.Result{}, nil
	}
	vsNamespacedName := client.ObjectKey{
		Namespace: rlp.Namespace,
		Name:      vsName,
	}

	var vs istio.VirtualService
	if err := r.Client().Get(ctx, vsNamespacedName, &vs); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("no virtualservice found", "lookup name", vsNamespacedName.String())
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get VirutalService")
		return ctrl.Result{}, err
	}
	if err := r.reconcileWithVirtualService(ctx, &vs, &rlp); err != nil {

	}
	return ctrl.Result{}, nil
}

func (r *RateLimitPolicyReconciler) reconcileWithVirtualService(ctx context.Context, vs *istio.VirtualService, rlp *apimv1alpha1.RateLimitPolicy) error {
	logger := r.Logger()

	_, ok := vs.Annotations[VSAnnotationRateLimitPolicy]
	if !ok {
		rlpNamespacedName := client.ObjectKeyFromObject(rlp)
		vs.Annotations[VSAnnotationRateLimitPolicy] = rlpNamespacedName.String()
		if err := r.Client().Update(ctx, vs); err != nil {
			logger.Error(err, "failed to add ratelimitpolicy annotation to virtualservice")
			return err
		}
		logger.V(1).Info("successfully added ratelimitpolicy annotation to virtualservice")
	}

	// TODO(rahulanand16nov): store context of virtualservice in RLP's status block and manage envoy patches.
	if err := r.reconcileRateLimitFilters(ctx, vs, rlp); err != nil {
		logger.Error(err, "failed to reconcile ratelimit filters")
		return err
	}

	if err := r.reconcileRateLimitActions(ctx, vs, rlp); err != nil {
		return err
	}
	logger.Info("successfully reconciled ratelimitpolicy using attached virtualservice")
	return nil
}

func (r *RateLimitPolicyReconciler) reconcileRateLimitActions(ctx context.Context, vs *istio.VirtualService, rlp *apimv1alpha1.RateLimitPolicy) error {
	logger := r.Logger()

	// TODO(rahulanand16nov): right now patch only applies to kuadrant-gateway but ideally should choose from virtualservice.
	for _, host := range vs.Spec.Hosts {
		for _, route := range rlp.Spec.Routes {
			isRouteMatch := doesRouteMatch(vs.Spec.Http, route.Name)
			if !isRouteMatch {
				logger.V(1).Info("no match found for route", "route name", route.Name)
				continue
			}

			vHostName := host + ":80" // Istio names virtual host in this format: host + port
			routePatch := routeRateLimitsPatch(vHostName, route.Name, route.Actions, route.Stage)

			if err := controllerutil.SetOwnerReference(rlp, routePatch, r.Client().Scheme()); err != nil {
				logger.Error(err, "failed to add owner ref to route-level ratelimits patch")
				return err
			}
			if err := r.Client().Create(ctx, routePatch); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					logger.Error(err, "failed to create route specific rate limit patch %s", routePatch.Name)
					return err
				}
			}
			logger.Info("created route level ratelimits patch", "name", routePatch.Name)
		}
	}
	return nil
}

func (r *RateLimitPolicyReconciler) reconcileRateLimitFilters(ctx context.Context, vs *istio.VirtualService, rlp *apimv1alpha1.RateLimitPolicy) error {
	logger := r.Logger()

	for _, gw := range vs.Spec.Gateways {
		gwKey := gatewayNameToObjectKey(gw, vs.Namespace) // Istio defaults to vs namespace
		rateLimitPatch := rateLimitInitialPatch(gwKey)

		if err := controllerutil.SetOwnerReference(rlp, rateLimitPatch, r.Client().Scheme()); err != nil {
			logger.Error(err, "failed to add owner ref to RateLimit filters patch")
			return err
		}
		if err := r.Client().Create(ctx, rateLimitPatch); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				logger.Error(err, "failed to patch-in RateLimit filters")
				return err
			}
		}
		logger.Info("created RateLimit filters patch", "gateway", gwKey.String())
	}
	logger.Info("successfully reconciled ratelimit filters")
	return nil
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
// - Change cluster name pointing to Limitador in kuadrant-system namespace (temp)
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
							"cluster_name": PatchedLimitadorClusterName,
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

	// Eventually, this should be dropped since it's a temp-fix for Kuadrant/limitador#53
	clusterPatch := networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
		ApplyTo: networkingv1alpha3.EnvoyFilter_CLUSTER,
		Match: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch{
			Context: networkingv1alpha3.EnvoyFilter_GATEWAY,
			ObjectTypes: &networkingv1alpha3.EnvoyFilter_EnvoyConfigObjectMatch_Cluster{
				Cluster: &networkingv1alpha3.EnvoyFilter_ClusterMatch{
					Name: DefaultLimitadorClusterName,
				},
			},
		},
		Patch: &networkingv1alpha3.EnvoyFilter_Patch{
			Operation: networkingv1alpha3.EnvoyFilter_Patch_MERGE,
			Value: &types.Struct{
				Fields: map[string]*types.Value{
					"name": {
						Kind: &types.Value_StringValue{
							StringValue: PatchedLimitadorClusterName,
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
	stageValue := apimv1alpha1.RateLimit_Stage_value[apimv1alpha1.RateLimitStage_PREAUTH]
	if stage == apimv1alpha1.RateLimitStage_POSTAUTH {
		stageValue = apimv1alpha1.RateLimit_Stage_value[apimv1alpha1.RateLimitStage_POSTAUTH]
	}

	patchUnstructured := map[string]interface{}{
		"operation": "MERGE",
		"value": map[string]interface{}{
			"route": map[string]interface{}{
				"rate_limits": []map[string]interface{}{
					{
						"stage":   stageValue,
						"actions": "ACTIONS", // this is replaced with actual value below
					},
				},
			},
		},
	}

	patchRaw, _ := json.Marshal(patchUnstructured)
	stringPatch := string(patchRaw)

	jsonActions, _ := json.Marshal(actions)
	// A nice trick to make it easier add-in actions
	stringPatch = strings.Replace(stringPatch, "\"ACTIONS\"", string(jsonActions), 1)
	patchRaw = []byte(stringPatch)

	Patch := &networkingv1alpha3.EnvoyFilter_Patch{}
	err := Patch.UnmarshalJSON(patchRaw)
	if err != nil {
		panic(err)
	}

	if stage == apimv1alpha1.RateLimitStage_BOTH {
		routeCopy := Patch.DeepCopy().Value.Fields["route"]
		updatedRateLimits := routeCopy.GetStructValue().Fields["rate_limits"].GetListValue().Values
		updatedRateLimits[0].GetStructValue().Fields["stage"] = &types.Value{
			Kind: &types.Value_NumberValue{
				NumberValue: float64(stageValue ^ 1), // toggle between 1/0
			},
		}

		finalRoute := Patch.Value.Fields["route"]
		ogRateLimits := finalRoute.GetStructValue().Fields["rate_limits"].GetListValue().Values
		finalRateLimits := append(ogRateLimits, updatedRateLimits[0])
		finalRoute.GetStructValue().Fields["rate_limits"].GetListValue().Values = finalRateLimits
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
		Patch: Patch,
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

func alwaysUpdateRateLimit(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*v1alpha1.RateLimit)
	if !ok {
		return false, fmt.Errorf("%T is not a *networkingv1beta1.RateLimit", existingObj)
	}
	desired, ok := desiredObj.(*v1alpha1.RateLimit)
	if !ok {
		return false, fmt.Errorf("%T is not a *networkingv1beta1.RateLimit", desiredObj)
	}

	existing.Spec = desired.Spec
	return true, nil
}

func (r *RateLimitPolicyReconciler) reconcileLimits(ctx context.Context, rlp *apimv1alpha1.RateLimitPolicy) error {
	logger := r.Logger()

	// create the RateLimit resource
	for i, rlSpec := range rlp.Spec.Limits {
		ratelimitfactory := limitador.RateLimitFactory{
			Key: client.ObjectKey{
				Name:      fmt.Sprintf("%s-limit-%d", rlp.Name, i+1),
				Namespace: rlp.Namespace,
			},
			Conditions: rlSpec.Conditions,
			MaxValue:   rlSpec.MaxValue,
			Namespace:  rlSpec.Namespace,
			Variables:  rlSpec.Variables,
			Seconds:    rlSpec.Seconds,
		}

		ratelimit := ratelimitfactory.RateLimit()
		if err := controllerutil.SetOwnerReference(rlp, ratelimit, r.Client().Scheme()); err != nil {
			logger.Error(err, "failed to add owner ref to RateLimit resource")
			return err
		}
		err := r.BaseReconciler.ReconcileResource(ctx, &v1alpha1.RateLimit{}, ratelimit, alwaysUpdateRateLimit)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "ReconcileResource failed to create/update RateLimit resource")
			return err
		}
	}
	logger.Info("successfully created/updated RateLimit resources")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RateLimitPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.RateLimitPolicy{}).
		Complete(r)
}
