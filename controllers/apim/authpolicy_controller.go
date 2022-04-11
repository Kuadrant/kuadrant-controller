package apim

import (
	"context"

	"github.com/go-logr/logr"
	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	secv1beta1types "istio.io/api/security/v1beta1"
	"istio.io/api/type/v1beta1"
	secv1beta1resources "istio.io/client-go/pkg/apis/security/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// AuthPolicyReconciler reconciles a AuthPolicy object
type AuthPolicyReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=authpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete

func (r *AuthPolicyReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("AuthPolicy", req.NamespacedName)
	logger.Info("Reconciling AuthPolicy")
	ctx := logr.NewContext(eventCtx, logger)

	var ap apimv1alpha1.AuthPolicy
	if err := r.Client().Get(ctx, req.NamespacedName, &ap); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("no AuthPolicy found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get AuthPolicy")
		return ctrl.Result{}, err
	}

	if err := r.reconcileAuthPolicy(ctx, &ap); err != nil {
		logger.Error(err, "failed to reconcile AuthPolicy")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// IstioAuthPolicy generates Istio's AuthorizationPolicy using Kuadrant's AuthPolicy
func (r *AuthPolicyReconciler) reconcileAuthPolicy(ctx context.Context, ap *apimv1alpha1.AuthPolicy) error {
	logger := logr.FromContext(ctx)

	httpRoute, err := r.fetchHTTPRoute(ctx, ap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("targetRef HTTPRoute not found")
			return nil
		}
		return err
	}

	for _, parentRef := range httpRoute.Spec.ParentRefs {
		gwNamespace := httpRoute.Namespace // consider gateway local if namespace is not given
		if parentRef.Namespace != nil {
			gwNamespace = string(*parentRef.Namespace)
		}
		gwName := string(parentRef.Name)

		// convert []Rule  to []*Rule
		rulePtrSlice := []*secv1beta1types.Rule{}
		for idx := range ap.Spec.Rules {
			rulePtrSlice = append(rulePtrSlice, &ap.Spec.Rules[idx])
		}

		action_int := secv1beta1types.AuthorizationPolicy_Action_value[string(ap.Spec.Action)]

		authPolicy := secv1beta1resources.AuthorizationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getAuthPolicyName(gwName, HTTPRouteNamePrefix, httpRoute.Name),
				Namespace: gwNamespace,
			},
			Spec: secv1beta1types.AuthorizationPolicy{
				Action: secv1beta1types.AuthorizationPolicy_Action(action_int),
				Rules:  rulePtrSlice,
				ActionDetail: &secv1beta1types.AuthorizationPolicy_Provider{
					Provider: &ap.Spec.Provider,
				},
				Selector: &v1beta1.WorkloadSelector{
					MatchLabels: map[string]string{}, // TODO(rahulanand16nov): fetch from gateway
				},
			},
		}

		err := r.ReconcileResource(ctx, &secv1beta1resources.AuthorizationPolicy{}, &authPolicy, alwaysUpdateAuthPolicy)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "ReconcileResource failed to create/update AuthorizationPolicy resource")
			return err
		}
	}

	return nil
}

// fetchHTTPRoute fetches the HTTPRoute described in targetRef *within* AuthPolicy's namespace.
func (r *AuthPolicyReconciler) fetchHTTPRoute(ctx context.Context, ap *apimv1alpha1.AuthPolicy) (*gatewayapiv1alpha2.HTTPRoute, error) {
	logger := logr.FromContext(ctx)

	key := client.ObjectKey{
		Name:      string(ap.Spec.TargetRef.Name),
		Namespace: ap.Namespace,
	}

	httpRoute := &gatewayapiv1alpha2.HTTPRoute{}
	err := r.Client().Get(ctx, key, httpRoute)
	logger.V(1).Info("fetchHTTPRoute", "httpRoute", key, "err", err)
	if err != nil {
		return nil, err
	}

	return httpRoute, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	HTTPRouteEventMapper := &HTTPRouteEventMapper{
		Logger: r.Logger().WithName("httpRouteHandler"),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.AuthPolicy{}).
		Watches(
			&source.Kind{Type: &gatewayapiv1alpha2.HTTPRoute{}},
			handler.EnqueueRequestsFromMapFunc(HTTPRouteEventMapper.Map),
		).
		Complete(r)
}
