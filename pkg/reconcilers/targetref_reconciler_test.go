//go:build unit

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

package reconcilers

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/kuadrant-controller/pkg/log"
)

func TestFetchValidGateway(t *testing.T) {
	var (
		namespace = "operator-unittest"
		gwName    = "my-gateway"
	)
	baseCtx := context.Background()
	ctx := logr.NewContext(baseCtx, log.Log)

	s := scheme.Scheme
	err := appsv1.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}
	err = gatewayapiv1alpha2.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}

	existingGateway := &gatewayapiv1alpha2.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.networking.k8s.io/v1alpha2",
			Kind:       "Gateway",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: namespace,
		},
		Spec: gatewayapiv1alpha2.GatewaySpec{
			GatewayClassName: "istio",
		},
		Status: gatewayapiv1alpha2.GatewayStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{existingGateway}

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	clientAPIReader := fake.NewFakeClient(objs...)
	recorder := record.NewFakeRecorder(1000)

	baseReconciler := NewBaseReconciler(cl, s, clientAPIReader, log.Log, recorder)
	targetRefReconciler := TargetRefReconciler{
		BaseReconciler: baseReconciler,
	}

	key := client.ObjectKey{Name: gwName, Namespace: namespace}

	res, err := targetRefReconciler.FetchValidGateway(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	if res == nil {
		t.Fatal("res is nil")
	}

	if !reflect.DeepEqual(res.Spec, existingGateway.Spec) {
		t.Fatal("res spec not as expected")
	}
}

func TestFetchValidHTTPRoute(t *testing.T) {
	var (
		namespace = "operator-unittest"
		routeName = "my-route"
	)
	baseCtx := context.Background()
	ctx := logr.NewContext(baseCtx, log.Log)

	s := scheme.Scheme
	err := appsv1.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}
	err = gatewayapiv1alpha2.AddToScheme(s)
	if err != nil {
		t.Fatal(err)
	}

	existingRoute := &gatewayapiv1alpha2.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "gateway.networking.k8s.io/v1alpha2",
			Kind:       "HTTPRoute",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: gatewayapiv1alpha2.HTTPRouteSpec{
			CommonRouteSpec: gatewayapiv1alpha2.CommonRouteSpec{
				ParentRefs: []gatewayapiv1alpha2.ParentRef{
					{
						Name: "gwName",
					},
				},
			},
		},
		Status: gatewayapiv1alpha2.HTTPRouteStatus{
			RouteStatus: gatewayapiv1alpha2.RouteStatus{
				Parents: []gatewayapiv1alpha2.RouteParentStatus{
					{
						ParentRef: gatewayapiv1alpha2.ParentRef{
							Name: "gwName",
						},
						Conditions: []metav1.Condition{
							{
								Type:   "Accepted",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{existingRoute}

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	clientAPIReader := fake.NewFakeClient(objs...)
	recorder := record.NewFakeRecorder(1000)

	baseReconciler := NewBaseReconciler(cl, s, clientAPIReader, log.Log, recorder)
	targetRefReconciler := TargetRefReconciler{
		BaseReconciler: baseReconciler,
	}

	key := client.ObjectKey{Name: routeName, Namespace: namespace}

	res, err := targetRefReconciler.FetchValidHTTPRoute(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	if res == nil {
		t.Fatal("res is nil")
	}

	if !reflect.DeepEqual(res.Spec, existingRoute.Spec) {
		t.Fatal("res spec not as expected")
	}
}
