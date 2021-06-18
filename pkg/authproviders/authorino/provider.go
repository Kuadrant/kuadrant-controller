package authorino

import (
	"context"

	"github.com/go-logr/logr"
	authorino "github.com/kuadrant/authorino/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

type AuthorinoProvider struct {
	*reconcilers.BaseReconciler
	logger logr.Logger
}

// +kubebuilder:rbac:groups=config.authorino.3scale.net,resources=services,verbs=get;list;watch;create;update;patch;delete

func New(baseReconciler *reconcilers.BaseReconciler) *AuthorinoProvider {
	utilruntime.Must(authorino.AddToScheme(baseReconciler.Scheme()))

	return &AuthorinoProvider{
		BaseReconciler: baseReconciler,
		logger:         ctrl.Log.WithName("kuadrant").WithName("authprovider").WithName("authorino"),
	}
}

func (a *AuthorinoProvider) Logger() logr.Logger {
	return a.logger
}

func (a *AuthorinoProvider) Reconcile(ctx context.Context, apip *networkingv1beta1.APIProduct) (ctrl.Result, error) {
	log := a.Logger().WithValues("apiproduct", client.ObjectKeyFromObject(apip))
	log.V(1).Info("Reconcile")

	serviceConfig, err := APIProductToServiceConfigs(apip)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Currently, authorino will not be updated
	// TODO implement mutator to reconcile desired object -> existing object
	err = a.ReconcileAuthorinoService(ctx, serviceConfig, reconcilers.CreateOnlyMutator)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (a *AuthorinoProvider) ReconcileAuthorinoService(ctx context.Context, desired *authorino.Service, mutatefn reconcilers.MutateFn) error {
	return a.ReconcileResource(ctx, &authorino.Service{}, desired, mutatefn)
}

func APIProductToServiceConfigs(apip *networkingv1beta1.APIProduct) (*authorino.Service, error) {
	serviceConfig := &authorino.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apip.Name + apip.Namespace,
			Namespace: istioprovider.KuadrantNamespace,
		},
		Spec: authorino.ServiceSpec{
			Hosts:         apip.Spec.Routing.Hosts,
			Identity:      nil,
			Metadata:      nil,
			Authorization: nil,
		},
	}

	for _, securityScheme := range apip.Spec.SecurityScheme {
		identity := authorino.Identity{
			Name:           securityScheme.Name,
			Credentials:    authorino.Credentials{},
			OAuth2:         nil,
			Oidc:           nil,
			APIKey:         nil,
			KubernetesAuth: nil,
		}
		if securityScheme.APIKeyAuth != nil {
			apikey := authorino.Identity_APIKey{
				LabelSelectors: securityScheme.APIKeyAuth.CredentialSource.LabelSelectors,
			}
			identity.Credentials.In = authorino.Credentials_In(securityScheme.APIKeyAuth.Location)
			identity.Credentials.KeySelector = securityScheme.APIKeyAuth.Name
			identity.APIKey = &apikey
		} else if securityScheme.OpenIDConnectAuth != nil {
			identity.Oidc = &authorino.Identity_OidcConfig{
				Endpoint: securityScheme.OpenIDConnectAuth.URL,
			}
		}
		serviceConfig.Spec.Identity = append(serviceConfig.Spec.Identity, &identity)
	}
	return serviceConfig, nil
}

func (a *AuthorinoProvider) Delete(ctx context.Context, apip *networkingv1beta1.APIProduct) (err error) {
	log := a.Logger().WithValues("apiproduct", client.ObjectKeyFromObject(apip))
	log.V(1).Info("Delete")
	return nil
}

func (a *AuthorinoProvider) Status(ctx context.Context, apip *networkingv1beta1.APIProduct) (ready bool, err error) {
	log := a.Logger().WithValues("apiproduct", client.ObjectKeyFromObject(apip))
	log.V(1).Info("Status")

	// Right now, we just try to get all the objects that should have been created, and check their status.
	// If any object is missing/not-created, Status returns false.
	serviceConfig, err := APIProductToServiceConfigs(apip)
	if err != nil {
		return false, err
	}

	existingServiceConfig := &authorino.Service{}
	err = a.Client().Get(ctx, client.ObjectKeyFromObject(serviceConfig), existingServiceConfig)
	if err != nil && errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// TODO(jmprusi): No status in authorino serviceConfig objects, yet.
	//if !existingServiceConfig.Status.Ready {
	//	return false, nil
	//}

	return true, nil
}
