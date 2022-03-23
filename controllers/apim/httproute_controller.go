package apim

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kuadrant/kuadrant-controller/pkg/log"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
	securityv1beta1 "istio.io/api/security/v1beta1"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	gatewayapi_v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const HTTPRouteNamePrefix = "hr"

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;update;patch

// HTTPRouteReconciler reconciles Gateway API's HTTPRoute object
type HTTPRouteReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

func (r *HTTPRouteReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("HTTPRoute", req.NamespacedName)
	ctx := logr.NewContext(eventCtx, logger)

	httproute := gatewayapi_v1alpha2.HTTPRoute{}
	if err := r.Client().Get(ctx, req.NamespacedName, &httproute); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get HTTPRoute")
		return ctrl.Result{}, err
	}

	if _, present := httproute.GetAnnotations()[KuadrantRateLimitPolicyAnnotation]; present {
		if err := SendSignal(ctx, r.Client(), &httproute); err != nil {
			logger.Error(err, "failed to send signal to RateLimitPolicy")
			return ctrl.Result{}, err
		}
	}

	// TODO(rahulanand16nov): handle HTTPRoute deletion for AuthPolicy
	// check if this httproute has to be protected or not.
	_, present := httproute.GetAnnotations()[KuadrantAuthProviderAnnotation]
	if !present {
		for _, parentRef := range httproute.Spec.ParentRefs {
			gwNamespace := httproute.Namespace // consider gateway local if namespace is not given
			if parentRef.Namespace != nil {
				gwNamespace = string(*parentRef.Namespace)
			}
			gwName := string(parentRef.Name)
			authObjKey := types.NamespacedName{
				// Prefix is added to differentiate between routing objects
				Name:      getAuthPolicyName(gwName, HTTPRouteNamePrefix, httproute.Name),
				Namespace: gwNamespace,
			}

			authPolicy := istiosecurityv1beta1.AuthorizationPolicy{}
			if err := r.Client().Get(context.Background(), authObjKey, &authPolicy); err != nil {
				// no annotation but authpolicy exist means annotation was removed.
				if !apierrors.IsNotFound(err) {
					logger.Error(err, "failed to check AuthorizationPolicy existence")
				}
				return ctrl.Result{}, nil // this httproute is not protected.
			}

			// Orphan AuthorizationPolicy exists
			if err := r.Client().Delete(context.Background(), &authPolicy); err != nil {
				logger.Error(err, "failed to delete orphan authorizationpolicy")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// reconcile authpolicy for the protected virtualservice
	if err := r.reconcileAuthPolicy(ctx, logger, &httproute); err != nil {
		logger.Error(err, "failed to reconcile AuthorizationPolicy")
		return ctrl.Result{}, err
	}
	logger.Info("successfully reconciled AuthorizationPolicy")
	return ctrl.Result{}, nil
}

func (r *HTTPRouteReconciler) reconcileAuthPolicy(ctx context.Context, logger logr.Logger, hr *gatewayapi_v1alpha2.HTTPRoute) error {
	logger.Info("Reconciling AuthorizationPolicy")

	// annotation presence is already checked.
	providerName := hr.GetAnnotations()[KuadrantAuthProviderAnnotation]

	// pre-convert hostnames to string slice
	hosts := HostnamesToStrings(hr.Spec.Hostnames)

	// generate rules
	rules := []*securityv1beta1.Rule{}
	for _, httpRouteRule := range hr.Spec.Rules {
		rule := &securityv1beta1.Rule{
			To:   []*securityv1beta1.Rule_To{},
			When: []*securityv1beta1.Condition{},
		}
		for _, httpRouteMatch := range httpRouteRule.Matches {
			toRule := &securityv1beta1.Rule_To{
				Operation: &securityv1beta1.Operation{},
			}

			toRule.Operation.Hosts = hosts

			path := *httpRouteMatch.Path.Value
			if *httpRouteMatch.Path.Type == gatewayapi_v1alpha2.PathMatchPathPrefix {
				path += "*"
			}
			toRule.Operation.Paths = append(toRule.Operation.Paths, path)

			if httpRouteMatch.Method != nil {
				toRule.Operation.Methods = append(toRule.Operation.Methods, string(*httpRouteMatch.Method))
			}

			rule.To = append(rule.To, toRule)

			conditions := []*securityv1beta1.Condition{}
			// convert header matchs into conditions
			for _, header := range httpRouteMatch.Headers {
				// skip regular expression and empty header value
				if *header.Type == gatewayapi_v1alpha2.HeaderMatchRegularExpression || header.Value == "" {
					continue
				}

				headerCondition := &securityv1beta1.Condition{
					Key:    fmt.Sprintf("request.headers[%s]", header.Name),
					Values: []string{header.Value},
				}
				conditions = append(conditions, headerCondition)
			}
			// convert query params into condtions
			for _, queryParam := range httpRouteMatch.QueryParams {
				if *queryParam.Type == gatewayapi_v1alpha2.QueryParamMatchRegularExpression || queryParam.Value == "" {
					continue
				}

				queryCondition := &securityv1beta1.Condition{
					Key:    "request.headers[query_string]",
					Values: []string{queryParam.Value},
				}
				conditions = append(conditions, queryCondition)
			}
			rule.When = conditions
		}
		rules = append(rules, rule)
	}

	authPolicySpec := securityv1beta1.AuthorizationPolicy{
		Rules:  rules,
		Action: securityv1beta1.AuthorizationPolicy_CUSTOM,
		ActionDetail: &securityv1beta1.AuthorizationPolicy_Provider{
			Provider: &securityv1beta1.AuthorizationPolicy_ExtensionProvider{
				Name: providerName,
			},
		},
	}

	for _, parentRef := range hr.Spec.ParentRefs {
		gwNamespace := hr.Namespace // consider gateway local if namespace is not given
		if parentRef.Namespace != nil {
			gwNamespace = string(*parentRef.Namespace)
		}
		gwName := string(parentRef.Name)

		authPolicy := istiosecurityv1beta1.AuthorizationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getAuthPolicyName(gwName, HTTPRouteNamePrefix, hr.Name),
				Namespace: gwNamespace,
			},
			Spec: authPolicySpec,
		}

		err := r.ReconcileResource(ctx, &istiosecurityv1beta1.AuthorizationPolicy{}, &authPolicy, alwaysUpdateAuthPolicy)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Error(err, "ReconcileResource failed to create/update AuthorizationPolicy resource")
			return err
		}
	}

	logger.Info("successfully created/updated AuthorizationPolicy resource(s)")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HTTPRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayapi_v1alpha2.HTTPRoute{}, builder.WithPredicates(RoutingPredicate())).
		WithLogger(log.Log). // use base logger, the manager will add prefixes for watched sources
		Complete(r)
}

func HostnamesToStrings(hostnames []gatewayapi_v1alpha2.Hostname) []string {
	hosts := []string{}
	for idx := range hostnames {
		hosts = append(hosts, string(hostnames[idx]))
	}
	return hosts
}
