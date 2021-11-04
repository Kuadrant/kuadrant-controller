package gatewayapiprovider

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type GatewayAPIProvider struct {
	*reconcilers.BaseReconciler
}

func (gap *GatewayAPIProvider) Reconcile(ctx context.Context, apip *networkingv1beta1.APIProduct) (ctrl.Result, error) {
	logger := logr.FromContext(ctx).WithName("ingressprovider").WithName("gatewayapi")
	// look for a gateway with the provided gateway name
	// if it has been marked for deletion need to clean up

	if apip.DeletionTimestamp != nil {
		logger.Info("deleted")
		return ctrl.Result{}, nil
	}

	gl := &gatewayapi.GatewayList{}
	if err := gap.Client().List(ctx, gl); err != nil {
		return ctrl.Result{}, err
	}
	var gateway gatewayapi.Gateway
	if apip.Spec.GatewayName != "" {
		for _, gate := range gl.Items {
			if gate.Name == apip.Spec.GatewayName {
				gateway = gate
				break
			}
		}
	}
	if gateway.Spec.GatewayClassName == gatewayapi.ObjectName("") {
		return ctrl.Result{}, fmt.Errorf("no gateway found with name %s ", apip.Spec.GatewayName)
	}
	// if there is more than one API in the API Product we add both backends to the http route
	// if there is a HTTPPathMatch provided in the API then add the path to each backend rules
	// TODO OAS parsing
	// TODO what if two APIs services are in different namespaces?
	r := &gatewayapi.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apip.Name,
			Namespace: apip.Namespace,
		},
	}
	group := gateway.GroupVersionKind().Group
	kind := gateway.GroupVersionKind().Kind
	r.Spec.ParentRefs = []gatewayapi.ParentRef{{
		Namespace: (*gatewayapi.Namespace)(&gateway.Namespace),
		Name:      gatewayapi.ObjectName(gateway.Name),
		Group:     (*gatewayapi.Group)(&group),
		Kind:      (*gatewayapi.Kind)(&kind),
	}}
	r.Spec.Hostnames = []gatewayapi.Hostname{}
	for _, h := range apip.Spec.Hosts {
		r.Spec.Hostnames = append(r.Spec.Hostnames, gatewayapi.Hostname(h))
	}
	r.Spec.Rules = []gatewayapi.HTTPRouteRule{}
	for _, api := range apip.Spec.APIs {
		// for each api create a new backend and matches section
		fetchedAPI := &networkingv1beta1.API{}
		if err := gap.Client().Get(ctx, client.ObjectKey{Namespace: api.Namespace, Name: api.Name}, fetchedAPI); err != nil {
			// TODO prob needs a status update on the API Product
			return ctrl.Result{}, err
		}

		backend := gatewayapi.HTTPRouteRule{
			BackendRefs: []gatewayapi.HTTPBackendRef{
				{
					BackendRef: gatewayapi.BackendRef{
						BackendObjectReference: gatewayapi.BackendObjectReference{
							Name: gatewayapi.ObjectName(fetchedAPI.Spec.Destination.ServiceReference.Name),
							Port: (*gatewayapi.PortNumber)(fetchedAPI.Spec.Destination.ServiceReference.Port),
						},
					},
				},
			},
		}
		matches := []gatewayapi.HTTPRouteMatch{}
		if fetchedAPI.Spec.Mappings.HTTPPathMatch != nil {
			matches = append(matches, gatewayapi.HTTPRouteMatch{
				Path: fetchedAPI.Spec.Mappings.HTTPPathMatch,
			})
		}
		backend.Matches = matches
		r.Spec.Rules = append(r.Spec.Rules, backend)
	}

	r.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: apip.APIVersion,
		Kind:       apip.Kind,
		Name:       apip.Name,
		UID:        apip.UID,
	}}
	gap.ReconcileResource(ctx, &gatewayapi.HTTPRoute{}, r, func(existingObj, desiredObj client.Object) (bool, error) {
		existing, ok := existingObj.(*gatewayapi.HTTPRoute)
		if !ok {
			return false, fmt.Errorf("%T is not a *gateway.HTTPRoute", existing)
		}
		desired, ok := desiredObj.(*gatewayapi.HTTPRoute)
		if !ok {
			return false, fmt.Errorf("%T is not a *gateway.HTTPRoute", desired)
		}

		updated := false
		if !reflect.DeepEqual(existing.Spec, desired.Spec) {
			existing.Spec = desired.Spec
			updated = true
		}

		return updated, nil
	})
	return ctrl.Result{}, nil
}
func (gap *GatewayAPIProvider) Status(ctx context.Context, apip *networkingv1beta1.APIProduct) (bool, error) {
	return true, nil
}
func (gap *GatewayAPIProvider) Delete(ctx context.Context, apip *networkingv1beta1.APIProduct) error {
	return nil
}

func New(baseReconciler *reconcilers.BaseReconciler) *GatewayAPIProvider {
	// add the gateway api objects to the scheme
	utilruntime.Must(gatewayapi.AddToScheme(baseReconciler.Scheme()))
	return &GatewayAPIProvider{BaseReconciler: baseReconciler}
}
