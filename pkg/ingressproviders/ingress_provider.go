package ingressproviders

import (
	"context"
	"os"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/gatewayapiprovider"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

type IngressProvider interface {
	Reconcile(ctx context.Context, apip *networkingv1beta1.APIProduct) (ctrl.Result, error)
	Status(ctx context.Context, apip *networkingv1beta1.APIProduct) (bool, error)
	Delete(ctx context.Context, apip *networkingv1beta1.APIProduct) error
}

// GetIngressProvider returns the IngressProvider desired
//
//	TODO: Either look for an ENV var or check the cluster capabilities
//
func GetIngressProvider(baseReconciler *reconcilers.BaseReconciler) IngressProvider {
	providerName := strings.ToLower(strings.TrimSpace(os.Getenv("KUADRANT_INGRESS_PROVIDER")))
	var provider IngressProvider
	switch providerName {
	case "gatewayapi":
		provider = gatewayapiprovider.New(baseReconciler)
	default:
		provider = istioprovider.New(baseReconciler)
	}
	return provider
}
