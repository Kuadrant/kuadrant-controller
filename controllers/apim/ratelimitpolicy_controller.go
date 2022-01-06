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

package apim

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/go-logr/logr"
	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	limitador "github.com/kuadrant/kuadrant-controller/pkg/ratelimitproviders/limitador"
	"github.com/kuadrant/kuadrant-controller/pkg/reconcilers"
)

// RateLimitPolicyReconciler reconciles a RateLimitPolicy object
type RateLimitPolicyReconciler struct {
	*reconcilers.BaseReconciler
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=ratelimitpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=ratelimitpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apim.kuadrant.io,resources=ratelimitpolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RateLimitPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *RateLimitPolicyReconciler) Reconcile(eventCtx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger().WithValues("RateLimitPolicy", req.NamespacedName)
	logger.Info("Reconciling RateLimitPolicy")
	ctx := logr.NewContext(eventCtx, logger)

	var rlp apimv1alpha1.RateLimitPolicy
	if err := r.Client().Get(ctx, req.NamespacedName, &rlp); err != nil {
		logger.Error(err, "failed to get RateLimitPolicy")
		return ctrl.Result{}, err
	}

	specResult, specErr := r.reconcileSpec(ctx, &rlp)
	logger.Info("spec reconcile done", "result", specResult, "error", specErr)
	if specErr == nil && specResult.Requeue {
		logger.Info("Reconciling not finished. Requeueing.")
		return specResult, nil
	}
	return ctrl.Result{}, nil
}

func (r *RateLimitPolicyReconciler) reconcileSpec(ctx context.Context, rlp *apimv1alpha1.RateLimitPolicy) (ctrl.Result, error) {
	logger := r.Logger()

	// create the RateLimit resource
	for i, rlSpec := range rlp.Spec.Limits {
		ratelimitfactory := limitador.RateLimitFactory{
			Key: types.NamespacedName{
				Name:      fmt.Sprintf("%s.limit-%d", rlp.Name, i+1),
				Namespace: rlp.Namespace,
			},
			Conditions: rlSpec.Conditions,
			MaxValue:   rlSpec.MaxValue,
			Namespace:  rlSpec.Namespace,
			Variables:  rlSpec.Variables,
			Seconds:    rlSpec.Seconds,
		}

		ratelimit := ratelimitfactory.RateLimit()

		if err := r.Client().Create(ctx, ratelimit); err != nil {
			logger.Error(err, "failed to create RateLimit resource")
			return ctrl.Result{}, err
		}
	}
	logger.Info("successfully created RateLimit resources")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RateLimitPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.RateLimitPolicy{}).
		Complete(r)
}
