package authproviders

import (
	"context"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/authproviders/authorino"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

//TODO(jmprusi): I have doubts about how useful this interface is going to be, as it depends on the IngressProvider to
//               be configured for this AuthProvider... Perhaps we should get rid of this.

type AuthProvider interface {
	Reconcile(ctx context.Context, apip *networkingv1beta1.APIProduct) (ctrl.Result, error)
	Status(ctx context.Context, apip *networkingv1beta1.APIProduct) (bool, error)
	Delete(ctx context.Context, apip *networkingv1beta1.APIProduct) error
}

// GetAuthProvider returns the desired authproviders
//
//	TODO: Either look for an ENV var or check the cluster capabilities
//
func GetAuthProvider(baseReconciler *reconcilers.BaseReconciler) AuthProvider {
	if os.Getenv("KUADRANT_INGRESS_PROVIDER") == "gatewayapi" {
		return authorino.NewWasmProvider(baseReconciler)
	}
	return authorino.New(baseReconciler)
}
