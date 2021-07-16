/*
 Copyright 2021 Red Hat, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package limitador

import (
	"context"
	"fmt"
	"reflect"

	limitadorv1alpha1 "github.com/3scale/limitador-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"github.com/kuadrant/kuadrant-controller/pkg/common"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

type Provider struct {
	*reconcilers.BaseReconciler
	logger logr.Logger
}

// +kubebuilder:rbac:groups=limitador.3scale.net,resources=ratelimits,verbs=get;list;watch;create;update;patch;delete

func New(baseReconciler *reconcilers.BaseReconciler) *Provider {
	utilruntime.Must(limitadorv1alpha1.AddToScheme(baseReconciler.Scheme()))

	return &Provider{
		BaseReconciler: baseReconciler,
		logger:         ctrl.Log.WithName("kuadrant").WithName("ratelimitprovider").WithName("limitador"),
	}
}

func (p *Provider) Logger() logr.Logger {
	return p.logger
}

func (p *Provider) Reconcile(ctx context.Context, apip *networkingv1beta1.APIProduct) (ctrl.Result, error) {
	log := p.Logger().WithValues("apiproduct", client.ObjectKeyFromObject(apip))
	log.V(1).Info("Reconcile")

	err := p.ReconcileRateLimit(ctx, p.basicInfraRateLimit(apip), rateLimitBasicMutator)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (p *Provider) Delete(ctx context.Context, apip *networkingv1beta1.APIProduct) error {
	log := p.Logger().WithValues("apiproduct", client.ObjectKeyFromObject(apip))
	log.V(1).Info("Delete")
	if apip.Spec.RateLimit == nil {
		return nil
	}

	desiredRateLimit := p.basicInfraRateLimit(apip)
	err := p.DeleteResource(ctx, desiredRateLimit)
	log.V(1).Info("Removing RateLimit", "ratelimit", client.ObjectKeyFromObject(desiredRateLimit), "error", err)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (p *Provider) Status(ctx context.Context, apip *networkingv1beta1.APIProduct) (bool, error) {
	log := p.Logger().WithValues("apiproduct", client.ObjectKeyFromObject(apip))
	log.V(1).Info("Status")
	if apip.Spec.RateLimit == nil {
		return true, nil
	}

	// Right now, we just try to get all the objects that should have been created, and check their status.
	// If any object is missing/not-created, Status returns false.
	desiredRateLimit := p.basicInfraRateLimit(apip)
	existing := &limitadorv1alpha1.RateLimit{}
	err := p.GetResource(ctx, client.ObjectKeyFromObject(desiredRateLimit), existing)
	if err != nil && apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func (p *Provider) basicInfraRateLimit(apip *networkingv1beta1.APIProduct) *limitadorv1alpha1.RateLimit {
	key := basicInfraRateLimitKey(apip)

	var rateLimit *limitadorv1alpha1.RateLimit
	if apip.Spec.RateLimit == nil {
		// create just with the key and tag to delete
		factory := RateLimitFactory{Key: key}
		rateLimit = factory.RateLimit()

		common.TagObjectToDelete(rateLimit)
	} else {
		factory := RateLimitFactory{
			Key:       key,
			Namespace: apip.RateLimitDomainName(),
			// Descriptor configured in EnvoyFilter in the rateLimitActionsEnvoyPatch method
			Conditions: []string{"generic_key == kuadrant"},
			Variables:  []string{},
			MaxValue:   int(apip.Spec.RateLimit.MaxValue),
			Seconds:    int(apip.Spec.RateLimit.Period),
		}

		rateLimit = factory.RateLimit()
	}

	return rateLimit
}

func (p *Provider) ReconcileRateLimit(ctx context.Context, desired *limitadorv1alpha1.RateLimit, mutatefn reconcilers.MutateFn) error {
	return p.ReconcileResource(ctx, &limitadorv1alpha1.RateLimit{}, desired, mutatefn)
}

func rateLimitBasicMutator(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*limitadorv1alpha1.RateLimit)
	if !ok {
		return false, fmt.Errorf("%T is not a *limitadorv1alpha1.RateLimit", existingObj)
	}
	desired, ok := desiredObj.(*limitadorv1alpha1.RateLimit)
	if !ok {
		return false, fmt.Errorf("%T is not a *limitadorv1alpha1.RateLimit", desiredObj)
	}

	updated := false
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		updated = true
	}

	return updated, nil
}

func basicInfraRateLimitKey(apip *networkingv1beta1.APIProduct) client.ObjectKey {
	// APIProduct name/namespace should be unique in the cluster
	return types.NamespacedName{Name: apip.GetName() + apip.GetNamespace(), Namespace: common.KuadrantNamespace}
}
