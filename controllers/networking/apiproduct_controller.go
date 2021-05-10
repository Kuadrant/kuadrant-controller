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

package networking

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kuadrant/kuadrant-controller/pkg/ingressproviders"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
)

// APIProductReconciler reconciles a APIProduct object
type APIProductReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	IngressProvider ingressproviders.IngressProvider
}

const (
	finalizerName = "kuadrant.io/apiproduct"
)

//+kubebuilder:rbac:groups=networking.kuadrant.io,resources=apiproducts;apis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.kuadrant.io,resources=apiproducts/status;apis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.kuadrant.io,resources=apiproducts/finalizers;apis/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *APIProductReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("apiproduct", req.NamespacedName)

	apip := networkingv1beta1.APIProduct{}
	err := r.Client.Get(ctx, req.NamespacedName, &apip)
	if err != nil && errors.IsNotFound(err) {
		log.Info("APIProduct Object has been deleted.")
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// APIProduct has been marked for deletion
	if apip.GetDeletionTimestamp() != nil && controllerutil.ContainsFinalizer(&apip, finalizerName) {
		// cleanup the Ingress objects.
		r.IngressProvider.Delete(ctx, apip)

		//Remove finalizer and update the object.
		controllerutil.RemoveFinalizer(&apip, finalizerName)
		err := r.Client.Update(ctx, &apip)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if !controllerutil.ContainsFinalizer(&apip, finalizerName) {
		controllerutil.AddFinalizer(&apip, finalizerName)
		err := r.Update(ctx, &apip)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Use the ingress provider to create the APIProduct.
	// If it does exists, Update it.
	err = r.IngressProvider.Create(ctx, apip)
	if err != nil && errors.IsAlreadyExists(err) {
		r.IngressProvider.Update(ctx, apip)
	} else {
		return ctrl.Result{}, err
	}

	// Check if the provider objects are set to Ready.
	ok, err := r.IngressProvider.Status(apip)
	if err != nil {
		return ctrl.Result{}, err
	}
	if ok {
		// TODO(jmprusi): Set the APIProduct object ready.
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIProductReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1beta1.APIProduct{}).
		Complete(r)
}
