package authproviders

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/authproviders/authorino"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AuthProvider interface {
	Create(ctx context.Context, apip v1beta1.APIProduct) error
	Update(ctx context.Context, apip v1beta1.APIProduct) error
	Delete(ctx context.Context, apip v1beta1.APIProduct) error
	Status(apip v1beta1.APIProduct) (bool, error)
}

// GetAuthProvider returns the desired authproviders
//
//	TODO: Either look for an ENV var or check the cluster capabilities
//
func GetAuthProvider(logr logr.Logger, client client.Client) AuthProvider {
	return authorino.New(logr, client)
}
