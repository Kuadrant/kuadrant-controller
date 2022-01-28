package apim

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	securityv1beta1 "istio.io/api/security/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"

	"github.com/go-logr/logr"
	"github.com/kuadrant/kuadrant-controller/pkg/log"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

const (
	KuadrantAuthProviderAnnotation = "kuadrant.io/auth-provider"
)

// VirtualServiceReconciler reconciles Istio's AuthorizationPolicy object
type VirtualServiceReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

func (r *VirtualServiceReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("VirtualService", req.NamespacedName)
	ctx := logr.NewContext(eventCtx, logger)

	virtualService := istionetworkingv1alpha3.VirtualService{}
	if err := r.Client().Get(ctx, req.NamespacedName, &virtualService); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get VirtualService")
		return ctrl.Result{}, err
	}

	if err := r.reconcileAuthPolicy(ctx, &virtualService); err != nil {
		logger.Error(err, "failed to reconcile AuthorizationPolicy")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func getAuthPolicyName(vsName string) string {
	return "source-virtualservice-" + vsName
}

func (r *VirtualServiceReconciler) reconcileAuthPolicy(ctx context.Context, vs *istionetworkingv1alpha3.VirtualService) error {
	logger := r.Logger()
	logger.Info("Reconciling AuthorizationPolicy")

	authPolicy := istiosecurityv1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getAuthPolicyName(vs.Name),
			Namespace: vs.Namespace,
		},
	}

	providerName, present := vs.GetAnnotations()[KuadrantAuthProviderAnnotation]
	if !present {
		return fmt.Errorf("Kuadrant auth-provider annotation not found")
	}

	// fill out the rules
	authToRules := []*securityv1beta1.Rule_To{}
	for _, httpRoute := range vs.Spec.Http {
		for idx, matchRequest := range httpRoute.Match {
			toRule := &securityv1beta1.Rule_To{
				Operation: &securityv1beta1.Operation{},
			}

			toRule.Operation.Hosts = vs.Spec.Hosts
			if normalizedURI := normalizeStringMatch(matchRequest.Uri); normalizedURI != "" {
				toRule.Operation.Paths = append(toRule.Operation.Paths, normalizedURI)
			}

			if normalizedMethod := normalizeStringMatch(matchRequest.Method); normalizedMethod != "" {
				// Looks like it's case-sensitive:
				// https://istio.io/latest/docs/reference/config/security/normalization/#1-method-not-in-upper-case
				method := strings.ToUpper(normalizedMethod)
				toRule.Operation.Methods = append(toRule.Operation.Methods, method)
			}

			// If there is only regex stringmatches then we'll have bunch of repeated To rules with
			// only same host filled into each. Following make sure only one field like that is present.
			operation := toRule.Operation
			if len(operation.Paths) == 0 && len(operation.Methods) == 0 && idx > 0 {
				continue
			}
			authToRules = append(authToRules, toRule)
		}
	}

	authPolicy.Spec = securityv1beta1.AuthorizationPolicy{
		Rules: []*securityv1beta1.Rule{{
			To: authToRules,
		}},
		Action: securityv1beta1.AuthorizationPolicy_CUSTOM,
		ActionDetail: &securityv1beta1.AuthorizationPolicy_Provider{
			Provider: &securityv1beta1.AuthorizationPolicy_ExtensionProvider{
				Name: providerName,
			},
		},
	}

	if err := controllerutil.SetOwnerReference(vs, &authPolicy, r.Client().Scheme()); err != nil {
		logger.Error(err, "failed to add owner ref to AuthorizationPolicy resource")
		return err
	}
	err := r.ReconcileResource(ctx, &istiosecurityv1beta1.AuthorizationPolicy{}, &authPolicy, alwaysUpdateAuthPolicy)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Error(err, "ReconcileResource failed to create/update AuthorizationPolicy resource")
		return err
	}

	logger.Info("successfully created/updated AuthorizationPolicy resources")
	return nil
}

func alwaysUpdateAuthPolicy(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*istiosecurityv1beta1.AuthorizationPolicy)
	if !ok {
		return false, fmt.Errorf("%T is not a *istiosecurityv1beta1.AuthorizationPolicy", existingObj)
	}
	desired, ok := desiredObj.(*istiosecurityv1beta1.AuthorizationPolicy)
	if !ok {
		return false, fmt.Errorf("%T is not a *istiosecurityv1beta1.AuthorizationPolicy", desiredObj)
	}

	existing.Spec = desired.Spec
	return true, nil
}

func normalizeStringMatch(sm *networkingv1alpha3.StringMatch) string {
	if prefix := sm.GetPrefix(); prefix != "" {
		return prefix + "*"
	}
	if exact := sm.GetExact(); exact != "" {
		return exact
	}
	// Regex string match is not supported because authpolicy doesn't as well.
	return ""
}

// VirtualServiceFilter allows generation of only relevant reconciliation request.
type VirtualServiceFilter struct {
	K8sClient client.Client
	Logger    logr.Logger
}

// Map contains filtering logic for virtualservice.
func (m *VirtualServiceFilter) Map(obj client.Object) []reconcile.Request {
	virtualServiceAnnotations := obj.GetAnnotations()
	_, present := virtualServiceAnnotations[KuadrantAuthProviderAnnotation]
	if !present {
		authObjKey := types.NamespacedName{
			Name:      getAuthPolicyName(obj.GetName()),
			Namespace: obj.GetNamespace(),
		}

		var authPolicy client.Object
		if err := m.K8sClient.Get(context.Background(), authObjKey, authPolicy); err != nil {
			// no annotation but authpolicy exist means annotation was removed
			if !apierrors.IsNotFound(err) {
				m.Logger.Error(err, "failed to check AuthorizationPolicy existence")
			}
			return nil // this virtualservice is not protected
		}

		// AuthorizationPolicy exists
		if err := m.K8sClient.Delete(context.Background(), authPolicy); err != nil {
			m.Logger.Error(err, "failed to delete orphan authorizationpolicy")
		}
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	virtualServiceFilter := VirtualServiceFilter{
		K8sClient: r.Client(),
		Logger:    r.Logger().WithName("AuthPolicyHandler"),
	}
	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			&source.Kind{Type: &istionetworkingv1alpha3.VirtualService{}},
			handler.EnqueueRequestsFromMapFunc(virtualServiceFilter.Map),
		).
		For(&istiosecurityv1beta1.AuthorizationPolicy{}).
		WithLogger(log.Log). // use base logger, the manager will add prefixes for watched sources
		Complete(r)
}
