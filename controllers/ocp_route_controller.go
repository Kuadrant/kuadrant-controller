package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	istioapinetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kuadrant/kuadrant-controller/pkg/common"
	kuadrantistio "github.com/kuadrant/kuadrant-controller/pkg/istio"
	"github.com/kuadrant/kuadrant-controller/pkg/log"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

const (
	KuadrantManagedLabel   = "kuadrant.io/managed"
	routeFinalizerName     = "kuadrant.io/route"
	RouteAnnotationService = "kuadrant.io/original-service"
	RouteAnnotationPort    = "kuadrant.io/original-port"
)

// +kubebuilder:rbac:groups=route.openshift.io,namespace=placeholder,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,namespace=placeholder,resources=routes/custom-host,verbs=create
// +kubebuilder:rbac:groups=route.openshift.io,namespace=placeholder,resources=routes/status,verbs=get

// RouteReconciler reconciles Openshift Route object
type RouteReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

func (r *RouteReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("Route", req.NamespacedName)
	ctx := logr.NewContext(eventCtx, logger)

	route := &routev1.Route{}
	if err := r.Client().Get(ctx, req.NamespacedName, route); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get Route")
		return ctrl.Result{}, err
	}

	if logger.V(1).Enabled() {
		jsonData, err := json.MarshalIndent(route, "", "  ")
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.V(1).Info(string(jsonData))
	}

	// Route has been marked for deletion
	if route.GetDeletionTimestamp() != nil && controllerutil.ContainsFinalizer(route, routeFinalizerName) {
		err := r.deleteVirtualService(ctx, route)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.deleteForwardingService(ctx, route)
		if err != nil {
			return ctrl.Result{}, err
		}

		restoreRoute(route)

		//Remove finalizer and update the object.
		controllerutil.RemoveFinalizer(route, routeFinalizerName)
		err = r.UpdateResource(ctx, route)
		logger.Info("Removing finalizer", "error", err)
		return ctrl.Result{Requeue: true}, err
	}

	// Ignore deleted resources, this can happen when foregroundDeletion is enabled
	// https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion
	if route.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(route, routeFinalizerName) {
		controllerutil.AddFinalizer(route, routeFinalizerName)
		err := r.UpdateResource(ctx, route)
		logger.Info("Adding finalizer", "error", err)
		return ctrl.Result{Requeue: true}, err
	}

	routeLabels := route.GetLabels()
	if kuadrantEnabled, ok := routeLabels[KuadrantManagedLabel]; !ok || kuadrantEnabled != "true" {
		// this route used to be kuadrant protected, not anymore
		err := r.deleteVirtualService(ctx, route)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.deleteForwardingService(ctx, route)
		if err != nil {
			return ctrl.Result{}, err
		}

		if updated := restoreRoute(route); updated {
			err := r.UpdateResource(ctx, route)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	forwardingSvc := desiredForwardingService(route)
	err := r.ReconcileResource(ctx, &corev1.Service{}, forwardingSvc, basicServiceMutator)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.enableKuadrantRoute(ctx, route)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredVS, err := r.desiredVirtualService(ctx, route)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.ReconcileResource(ctx, &istionetworkingv1alpha3.VirtualService{}, desiredVS, basicVirtualServiceMutator)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("successfully reconciled")
	return ctrl.Result{}, nil
}

func kuadrantRoutePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Lets filter for only Routes that have the kuadrant label and are enabled.
			if val, ok := e.Object.GetLabels()[KuadrantManagedLabel]; ok {
				enabled, _ := strconv.ParseBool(val)
				return enabled
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// In case the update object had the kuadrant label set to true, we need to reconcile it.
			if val, ok := e.ObjectOld.GetLabels()[KuadrantManagedLabel]; ok {
				enabled, _ := strconv.ParseBool(val)
				return enabled
			}
			// In case that route gets update by adding the label, and set to true, we need to reconcile it.
			if val, ok := e.ObjectNew.GetLabels()[KuadrantManagedLabel]; ok {
				enabled, _ := strconv.ParseBool(val)
				return enabled
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// If the object had the Kuadrant label, we need to handle its deletion
			_, ok := e.Object.GetLabels()[KuadrantManagedLabel]
			return ok
		},
	}
}

func desiredVSNameFromRoute(route *routev1.Route) string {
	return fmt.Sprintf("kuadrant-managed-%s-%s", route.Namespace, route.Name)
}

func desiredForwardingServiceNameFromRoute(route *routev1.Route) string {
	return fmt.Sprintf("kuadrant-managed-%s-%s", route.Namespace, route.Name)
}

func desiredForwardingService(route *routev1.Route) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredForwardingServiceNameFromRoute(route),
			Namespace: route.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeExternalName,
			// kuadrant gateway hardcoded
			ExternalName: "kuadrant-gateway.kuadrant-system.svc.cluster.local",
		},
	}
}

func (r *RouteReconciler) desiredVirtualService(ctx context.Context, route *routev1.Route) (*istionetworkingv1alpha3.VirtualService, error) {
	annotations := route.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	svcName := annotations[RouteAnnotationService]
	// the route and the backend service must be in the same namespace
	svcKey := client.ObjectKey{Name: svcName, Namespace: route.Namespace}
	svcPort, err := common.GetServicePortNumber(ctx, r.Client(), svcKey, annotations[RouteAnnotationPort])
	if err != nil {
		return nil, err
	}

	routePath := "/"
	if route.Spec.Path != "" {
		routePath = route.Spec.Path
	}
	httpRoutes := []*istioapinetworkingv1alpha3.HTTPRoute{
		{
			Name: route.Name,
			Match: []*istioapinetworkingv1alpha3.HTTPMatchRequest{
				{
					Uri: kuadrantistio.StringMatch(routePath, kuadrantistio.PathMatchPrefix),
				},
			},
			Route: []*istioapinetworkingv1alpha3.HTTPRouteDestination{
				{
					Destination: &istioapinetworkingv1alpha3.Destination{
						Host: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, route.Namespace),
						Port: &istioapinetworkingv1alpha3.PortSelector{
							Number: uint32(svcPort),
						},
					},
				},
			},
		},
	}

	return &istionetworkingv1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VirtualService",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desiredVSNameFromRoute(route),
			// Hardcoded to the kuadrant istio ingress gateway
			Namespace: common.KuadrantNamespace,
		},
		Spec: istioapinetworkingv1alpha3.VirtualService{
			// Hardcoded to the kuadrant istio ingress gateway
			Gateways: []string{"kuadrant-gateway"},
			Hosts:    []string{route.Spec.Host},
			Http:     httpRoutes,
		},
	}, nil
}

func (r *RouteReconciler) enableKuadrantRoute(ctx context.Context, route *routev1.Route) error {
	annotations := route.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if _, ok := annotations[RouteAnnotationService]; !ok {
		annotations[RouteAnnotationService] = route.Spec.To.Name
		annotations[RouteAnnotationPort] = route.Spec.Port.TargetPort.String()
		route.SetAnnotations(annotations)

		// route new target will be the kuadrant's forwarding service
		// this service will be a headless external name service poiting to the kuadrant gateway
		route.Spec.To.Name = desiredForwardingServiceNameFromRoute(route)

		// HTTP port
		// TLS not supported yet
		route.Spec.Port.TargetPort = intstr.FromString("http2")
		err := r.UpdateResource(ctx, route)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *RouteReconciler) deleteForwardingService(ctx context.Context, route *routev1.Route) error {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredForwardingServiceNameFromRoute(route),
			Namespace: route.Namespace,
		},
	}
	if err := r.DeleteResource(ctx, service); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

func (r *RouteReconciler) deleteVirtualService(ctx context.Context, route *routev1.Route) error {
	vs := &istionetworkingv1alpha3.VirtualService{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VirtualService",
			APIVersion: "networking.istio.io/v1alpha3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desiredVSNameFromRoute(route),
			// Hardcoded to the kuadrant istio ingress gateway
			Namespace: common.KuadrantNamespace,
		},
	}

	if err := r.DeleteResource(ctx, vs); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

func restoreRoute(route *routev1.Route) bool {
	annotations := route.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	updated := false
	if _, ok := annotations[RouteAnnotationService]; ok {
		updated = true

		route.Spec.To.Name = annotations[RouteAnnotationService]
		delete(annotations, RouteAnnotationService)
		route.Spec.Port.TargetPort = intstr.Parse(annotations[RouteAnnotationPort])
		delete(annotations, RouteAnnotationPort)
		route.SetAnnotations(annotations)
	}

	return updated
}

func basicVirtualServiceMutator(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*istionetworkingv1alpha3.VirtualService)
	if !ok {
		return false, fmt.Errorf("%T is not a *istionetworkingv1alpha3.VirtualService", existingObj)
	}
	desired, ok := desiredObj.(*istionetworkingv1alpha3.VirtualService)
	if !ok {
		return false, fmt.Errorf("%T is not a *istionetworkingv1alpha3.VirtualService", desiredObj)
	}

	updated := false
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		updated = true
	}

	return updated, nil
}

func basicServiceMutator(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*corev1.Service)
	if !ok {
		return false, fmt.Errorf("%T is not a *corev1.Service", existingObj)
	}
	desired, ok := desiredObj.(*corev1.Service)
	if !ok {
		return false, fmt.Errorf("%T is not a *corev1.Service", desiredObj)
	}

	updated := false
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		updated = true
	}

	return updated, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&routev1.Route{}, builder.WithPredicates(kuadrantRoutePredicate())).
		WithLogger(log.Log). // use base logger, the manager will add prefixes for watched sources
		Complete(r)
}
