package authorino

import (
	"context"

	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders/istioprovider"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	authorino "github.com/kuadrant/authorino/api/v1beta1"
	"github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AuthorinoProvider struct {
	Log       logr.Logger
	K8sClient client.Client
}

// +kubebuilder:rbac:groups=config.authorino.3scale.net,resources=services,verbs=get;list;watch;create;update;patch;delete

func New(logger logr.Logger, client client.Client) *AuthorinoProvider {
	utilruntime.Must(authorino.AddToScheme(client.Scheme()))

	return &AuthorinoProvider{
		Log:       logger,
		K8sClient: client,
	}
}
func (a *AuthorinoProvider) Create(ctx context.Context, apip v1beta1.APIProduct) (err error) {

	for _, environment := range apip.Spec.Environments {
		service := authorino.Service{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      apip.Name + apip.Namespace + environment.Name,
				Namespace: istioprovider.KuadrantNamespace,
			},
			Spec: authorino.ServiceSpec{
				Hosts:         environment.Hosts,
				Identity:      nil,
				Metadata:      nil,
				Authorization: nil,
			},
			Status: authorino.ServiceStatus{
				Ready: false,
			},
		}

		err := a.K8sClient.Create(ctx, &service)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *AuthorinoProvider) Update(ctx context.Context, apip v1beta1.APIProduct) (err error) {
	return nil
}

func (a *AuthorinoProvider) Delete(ctx context.Context, apip v1beta1.APIProduct) (err error) {
	return nil
}

func (a *AuthorinoProvider) Status(apip v1beta1.APIProduct) (ready bool, err error) {
	return true, nil
}
