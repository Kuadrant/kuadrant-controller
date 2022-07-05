package apim

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/common"
)

const (
	// RateLimitPolicy finalizer
	rateLimitPolicyFinalizer = "ratelimitpolicy.kuadrant.io/finalizer"
)

func (r *RateLimitPolicyReconciler) deleteNetworkResourceBackReference(ctx context.Context, rlp *apimv1alpha1.RateLimitPolicy) error {
	logger, _ := logr.FromContext(ctx)
	httpRoute, err := r.fetchHTTPRoute(ctx, rlp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("targetRef HTTPRoute not found")
			return nil
		}
		return err
	}

	// Reconcile the back reference:
	httpRouteAnnotations := httpRoute.GetAnnotations()
	if httpRouteAnnotations == nil {
		httpRouteAnnotations = map[string]string{}
	}

	if _, ok := httpRouteAnnotations[common.RateLimitPolicyBackRefAnnotation]; ok {
		delete(httpRouteAnnotations, common.RateLimitPolicyBackRefAnnotation)
		httpRoute.SetAnnotations(httpRouteAnnotations)
		err := r.UpdateResource(ctx, httpRoute)
		logger.V(1).Info("deleteNetworkResourceBackReference: update HTTPRoute", "httpRoute", client.ObjectKeyFromObject(httpRoute), "err", err)
		if err != nil {
			return err
		}
	}
	return nil
}
