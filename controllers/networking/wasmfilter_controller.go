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

package networking

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gogo/protobuf/types"
	networkingv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	"istio.io/api/extensions/v1alpha1"
	"istio.io/api/type/v1beta1"
	istioWasm "istio.io/client-go/pkg/apis/extensions/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	finalizerName = "kuadrant.io/wasmfilter"
)

// WASMFilterReconciler reconciles a WASMFilter object
type WASMFilterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*reconcilers.BaseReconciler
}

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=networking.kuadrant.io,resources=wasmfilters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.kuadrant.io,resources=wasmfilters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.kuadrant.io,resources=wasmfilters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WASMFilter object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *WASMFilterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	wf := &networkingv1alpha1.WASMFilter{}

	if err := r.Client.Get(ctx, req.NamespacedName, wf); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(wf, finalizerName) {
		controllerutil.AddFinalizer(wf, finalizerName)
		if err := r.BaseReconciler.UpdateResource(ctx, wf); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}
	// build the worklaod selector based on the http route assumes in same namespace
	route := &gatewayapi.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wf.Spec.HTTPRouteRef.Name,
			Namespace: wf.Namespace,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(route), route); err != nil {
		return ctrl.Result{}, err
	}
	// if being deleted remove all wasmplugins
	if wf.GetDeletionTimestamp() != nil && controllerutil.ContainsFinalizer(wf, finalizerName) {
		// Really needs a clean up too much repeating code right now POC
		for _, be := range wf.Spec.HTTPRouteRef.Backends {
			wp := &istioWasm.WasmPlugin{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-%s", wf.Namespace, wf.Spec.Type, be),
					Namespace: "kuadrant-system",
				},
			}
			if err := r.Client.Delete(ctx, wp); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(wf, finalizerName)
		if err := r.UpdateResource(ctx, wf); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	for _, rules := range route.Spec.Rules {
		for _, be := range rules.BackendRefs {
			if !backendTargetted(string(be.Name), wf.Spec.HTTPRouteRef.Backends) {
				// check if a wasm plugin exists and delete
				wp := &istioWasm.WasmPlugin{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s-%s", wf.Namespace, wf.Spec.Type, be.Name),
						Namespace: "kuadrant-system",
					},
				}
				if err := r.Client.Delete(ctx, wp); client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	workloads := map[string]map[string]string{}
	targettedBackends := filterBackends(wf, route)

	for _, tb := range targettedBackends {
		svc := &v1.Service{}
		svcKey := client.ObjectKey{Namespace: wf.Namespace, Name: string(tb.BackendObjectReference.Name)}
		if err := r.Client.Get(ctx, svcKey, svc); err != nil {
			return ctrl.Result{}, err
		}

		if svc.Spec.Selector == nil || len(svc.Spec.Selector) == 0 {
			return ctrl.Result{}, fmt.Errorf("failed to find a service selector")
		}
		workloads[svc.Name] = svc.Spec.Selector
	}
	// for each set of workloads/svc add a plugin

	for svcName, ws := range workloads {
		spec, err := r.buildPluginSpec(ctx, svcName, wf, ws)
		if err != nil {
			return ctrl.Result{}, err
		}
		wp := &istioWasm.WasmPlugin{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%s", wf.Namespace, wf.Spec.Type, svcName),
				Namespace: "kuadrant-system",
				Labels: map[string]string{
					"author": "kuadrant",
				},
			},
			Spec: *spec,
		}

		if err := r.BaseReconciler.ReconcileResource(ctx, &istioWasm.WasmPlugin{}, wp, wasmPluginBasicMutator); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func backendTargetted(backendName string, targeted []string) bool {
	for _, br := range targeted {
		// will also need to account for namespace
		if backendName == br {
			return true
		}
	}
	return false
}

func (r *WASMFilterReconciler) buildPluginSpec(ctx context.Context, svc string, wf *networkingv1alpha1.WASMFilter, sel map[string]string) (*v1alpha1.WasmPlugin, error) {
	var phase v1alpha1.PluginPhase
	var pluginConfig *types.Struct
	dynamicConfig, err := wf.DecodePluginConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to decode pluging config %s", err)
	}

	pluginConfig = &types.Struct{
		Fields: map[string]*types.Value{
			"operations": {
				Kind: &types.Value_ListValue{
					ListValue: &types.ListValue{Values: []*types.Value{}},
				},
			},
			"failure_mode_deny": {Kind: &types.Value_BoolValue{BoolValue: true}},
		},
	}
	switch wf.Spec.Type {
	case "kuadrant-authentication":

		authFields := map[string]*types.Value{
			"upstream_cluster": {Kind: &types.Value_StringValue{StringValue: "outbound|50051||authorino-authorization.kuadrant-system.svc.cluster.local"}},
		}

		if v, ok := dynamicConfig["excludePattern"]; ok {
			authFields["exclude_pattern"] = &types.Value{Kind: &types.Value_StringValue{StringValue: v.(string)}}
		}
		authconfig := &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{
			Fields: map[string]*types.Value{
				"Authenticate": {Kind: &types.Value_StructValue{
					StructValue: &types.Struct{Fields: authFields},
				}}},
		},
		}}
		vals := pluginConfig.Fields["operations"].GetListValue().Values
		vals = append(vals, authconfig)
		pluginConfig.Fields["operations"].GetListValue().Values = vals
		phase = v1alpha1.PluginPhase_AUTHN

		break
	case "kuadrant-authenticated-ratelimit":
		// HACK currently we cannot use the istio discoverd service as there is an issue with Limitador. So we have to add
		// limitador as envoy filter patch first
		ip := istioprovider.New(r.BaseReconciler)
		if err := ip.ReconcileClusterPatch(ctx, svc, wf.Namespace, sel); err != nil {
			return nil, err
		}
		rateLimitConfig := createRateLimitConfig(dynamicConfig)
		vals := pluginConfig.Fields["operations"].GetListValue().Values
		vals = append(vals, rateLimitConfig)
		pluginConfig.Fields["operations"].GetListValue().Values = vals
		phase = v1alpha1.PluginPhase_UNSPECIFIED_PHASE
		//	priority = &types.Int64Value{Value: 9}
		break
	case "kuadrant-ratelimit":
		// HACK currently we cannot use the istio discoverd service as there is an issue with Limitador. So we have to add
		// limitador as envoy filter patch first
		// TODO this is duplicated from above de dupe
		ip := istioprovider.New(r.BaseReconciler)
		if err := ip.ReconcileClusterPatch(ctx, svc, wf.Namespace, sel); err != nil {
			return nil, err
		}
		rateLimitConfig := createRateLimitConfig(dynamicConfig)
		vals := pluginConfig.Fields["operations"].GetListValue().Values
		vals = append(vals, rateLimitConfig)
		pluginConfig.Fields["operations"].GetListValue().Values = vals
		phase = v1alpha1.PluginPhase_AUTHN
	}

	pluginSpec := v1alpha1.WasmPlugin{
		Selector: &v1beta1.WorkloadSelector{
			MatchLabels: sel,
		},
		Url:          "https://raw.githubusercontent.com/rahulanand16nov/wasm-shim/dev/limitador-integration/deploy/wasm_shim.wasm",
		Phase:        phase,
		PluginConfig: pluginConfig,
		// bug can't seem to set priority via code works directly on the resource
		//Priority: &types.Int64Value{Value: int64(10)},
	}
	return &pluginSpec, nil
}

func createRateLimitConfig(wfConfig map[string]interface{}) *types.Value {

	rateLimitFields := map[string]*types.Value{
		"upstream_cluster": {Kind: &types.Value_StringValue{StringValue: "rate_limit_cluster"}},
	}

	if v, ok := wfConfig["domain"]; ok {
		rateLimitFields["domain"] = &types.Value{Kind: &types.Value_StringValue{StringValue: v.(string)}}
	}
	if v, ok := wfConfig["exclude_pattern"]; ok {
		rateLimitFields["exclude_pattern"] = &types.Value{Kind: &types.Value_StringValue{StringValue: v.(string)}}
	}

	rateLimitFields["actions"] = &types.Value{Kind: &types.Value_ListValue{
		ListValue: &types.ListValue{Values: []*types.Value{}},
	}}

	if v, ok := wfConfig["actions"]; ok {
		pluginActions := map[string]*types.Value{}
		wasmFilterActions := v.([]interface{})
		for _, a := range wasmFilterActions {
			for key, val := range a.(map[string]interface{}) {
				fmt.Println("actions is **** ", key, val)
				fields := map[string]*types.Value{}
				for k, kv := range val.(map[string]interface{}) {
					fields[k] = &types.Value{Kind: &types.Value_StringValue{StringValue: kv.(string)}}
				}
				pluginActions[key] = &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{
					Fields: fields,
				}}}
			}
			actionConfig := &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{
				Fields: pluginActions,
			},
			}}
			vals := rateLimitFields["actions"].GetListValue().Values
			rateLimitFields["actions"].GetListValue().Values = append(vals, actionConfig)
		}

	}

	rateLimitConfig := &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{
		Fields: map[string]*types.Value{
			"RateLimit": &types.Value{Kind: &types.Value_StructValue{
				StructValue: &types.Struct{Fields: rateLimitFields},
			}}},
	},
	}}
	return rateLimitConfig

}

func filterBackends(wf *networkingv1alpha1.WASMFilter, r *gatewayapi.HTTPRoute) []gatewayapi.HTTPBackendRef {
	found := []gatewayapi.HTTPBackendRef{}
	for _, rules := range r.Spec.Rules {
		for _, targetRef := range wf.Spec.HTTPRouteRef.Backends {
			for _, backendRef := range rules.BackendRefs {
				if targetRef == string(backendRef.Name) {
					found = append(found, backendRef)
				}
			}
		}
	}
	return found
}

// SetupWithManager sets up the controller with the Manager.
func (r *WASMFilterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	utilruntime.Must(istioWasm.AddToScheme(r.BaseReconciler.Scheme()))
	utilruntime.Must(gatewayapi.AddToScheme(r.BaseReconciler.Scheme()))
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.WASMFilter{}).
		Complete(r)
}

func wasmPluginBasicMutator(existingObj, desiredObj client.Object) (bool, error) {
	updated := false
	existing, ok := existingObj.(*istioWasm.WasmPlugin)
	if !ok {
		return false, fmt.Errorf("%T is not a istioWasm.WasmPlugin", existingObj)
	}
	desired, ok := desiredObj.(*istioWasm.WasmPlugin)
	if !ok {
		return false, fmt.Errorf("%T is not a *istioWasm.WasmPlugin", desiredObj)
	}
	if !reflect.DeepEqual(desired.Spec, existing.Spec) {
		existing.Spec = desired.Spec
		updated = true
	}

	return updated, nil
}
