package authorino

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/gogo/protobuf/types"
	authorino "github.com/kuadrant/authorino/api/v1beta1"
	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	"istio.io/api/extensions/v1alpha1"
	"istio.io/api/type/v1beta1"
	istioWasm "istio.io/client-go/pkg/apis/extensions/v1alpha1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WasmProvider struct {
	*reconcilers.BaseReconciler
}

func NewWasmProvider(b *reconcilers.BaseReconciler) *WasmProvider {
	utilruntime.Must(authorino.AddToScheme(b.Scheme()))
	utilruntime.Must(istioWasm.AddToScheme(b.Scheme()))
	return &WasmProvider{BaseReconciler: b}
}

func (wp *WasmProvider) Reconcile(ctx context.Context, apip *networkingv1beta1.APIProduct) (ctrl.Result, error) {
	logger := logr.FromContext(ctx).WithName("authprovider").WithName("wasm")
	logger.V(1).Info("Reconcile")
	if !apip.HasSecurity() {
		return ctrl.Result{}, nil
	}
	authConfig := buildAuthConfig(apip)
	if err := wp.ReconcileResource(ctx, &authorino.AuthConfig{}, authConfig, authorinoAuthConfigBasicMutator); err != nil {
		return ctrl.Result{}, err
	}
	// configure and create WASMPlugin
	// look up each service associated with the api product to get the labels

	// need a plugin for each workload unless they share a set of labels. For simplicity in the POC I have just created a separate one per service
	for _, api := range apip.Spec.APIs {
		//TODO remove hard coded config
		fetchedAPI := &networkingv1beta1.API{}
		if err := wp.Client().Get(ctx, client.ObjectKey{Namespace: api.Namespace, Name: api.Name}, fetchedAPI); err != nil {
			return ctrl.Result{}, err
		}
		selector, err := wp.buildSelector(ctx, fetchedAPI)
		if err != nil {
			return ctrl.Result{}, err
		}
		plugin := &istioWasm.WasmPlugin{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", api.Namespace, api.Name),
				Namespace: "kuadrant-system",
				Labels: map[string]string{
					"pluginAuthor": "kuadrant",
				},
			},
			// The hard coded pieces here are purly for POC reasons
			Spec: v1alpha1.WasmPlugin{
				Selector:   selector,
				Url:        "https://raw.githubusercontent.com/Kuadrant/wasm-shim/main/deploy/wasm_shim.wasm",
				PluginName: "apim-shim",
				PluginConfig: &types.Struct{
					Fields: map[string]*types.Value{
						"auth_cluster":      {Kind: &types.Value_StringValue{StringValue: "outbound|50051||authorino-authorization.kuadrant-system.svc.cluster.local"}},
						"failure_mode_deny": {Kind: &types.Value_BoolValue{BoolValue: true}},
					},
				},
			},
		}
		// TODO sort out owner refs
		if err := wp.ReconcileResource(ctx, &istioWasm.WasmPlugin{}, plugin, wasmPluginBasicMutator); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
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

func (wp *WasmProvider) buildSelector(ctx context.Context, api *networkingv1beta1.API) (*v1beta1.WorkloadSelector, error) {
	labels := map[string]string{}
	svc := &core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.Name,
			Namespace: api.Namespace,
		},
	}
	// we use the service selector here to find the labels to use for the workload selection. It may need to be specified by the user however as this could be too generic
	if err := wp.Client().Get(ctx, client.ObjectKeyFromObject(svc), svc); err != nil {
		return nil, err
	}
	if svc.Spec.Selector == nil || len(svc.Spec.Selector) == 0 {
		return nil, fmt.Errorf("cannot add wasm plugin for workload. No selectors specified on service")
	}
	for k, v := range svc.Spec.Selector {
		labels[k] = v
	}

	return &v1beta1.WorkloadSelector{
		MatchLabels: labels,
	}, nil
}

//TODO
func (wp *WasmProvider) Status(ctx context.Context, apip *networkingv1beta1.APIProduct) (bool, error) {
	return true, nil
}

//TODO
func (wp *WasmProvider) Delete(ctx context.Context, apip *networkingv1beta1.APIProduct) error {
	return nil
}
