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

package istioprovider

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kuadrant/kuadrant-controller/apis/networking/v1beta1"
	"istio.io/api/networking/v1alpha3"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//TODO: move the const to a proper place, or get it from config
const KuadrantNamespace = "kuadrant-system"

type IstioProvider struct {
	Log       logr.Logger
	K8sClient client.Client
}

// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
func New(logger logr.Logger, client client.Client) *IstioProvider {
	// Register the Istio Scheme into the client, so we can interact with istio objects.
	utilruntime.Must(istio.AddToScheme(client.Scheme()))

	// TODO: Create the gateway for Kuadrant
	// TODO: Add the proper config to the mesh for the extAuthz.

	return &IstioProvider{
		Log:       logger,
		K8sClient: client,
	}
}

func (is *IstioProvider) Create(ctx context.Context, api v1beta1.API) error {
	httpRoutes := is.getHTTPRoutes(api)

	virtualService := istio.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.GetFullName(),
			Namespace: KuadrantNamespace,
		},
		Spec: v1alpha3.VirtualService{
			Gateways: []string{"kuadrant-gateway"},
			Hosts:    api.GetHosts(),
			Http:     httpRoutes,
		},
	}
	is.addOwnerReference(virtualService, api)

	err := is.K8sClient.Create(ctx, &virtualService)
	if err != nil {
		return fmt.Errorf("failing to create Istio virtualservice for %s: %w", api.GetFullName(), err)
	}

	return nil
}

func (is *IstioProvider) addOwnerReference(virtualService istio.VirtualService, api v1beta1.API) {
	// TODO: the OwnerReference is not working across Namespaces
	virtualService.SetOwnerReferences(append(
		virtualService.GetOwnerReferences(),
		metav1.OwnerReference{
			APIVersion: api.APIVersion,
			Kind:       api.Kind,
			Name:       api.Name,
			UID:        api.UID,
		}))
}

func (is *IstioProvider) getHTTPRoutes(api v1beta1.API) []*v1alpha3.HTTPRoute {
	// Let's create the virtual service and the routes.
	httpRoutes := make([]*v1alpha3.HTTPRoute, len(api.Spec.Operations))
	for i, operation := range api.Spec.Operations {
		//TODO: Create the proper AuthorizationPolicy based on the security field.
		httpRoute := v1alpha3.HTTPRoute{
			Name: operation.ID,
			Match: []*v1alpha3.HTTPMatchRequest{
				{
					Uri: &v1alpha3.StringMatch{
						MatchType: &v1alpha3.StringMatch_Prefix{Prefix: operation.Path},
					},
					Method: &v1alpha3.StringMatch{
						MatchType: &v1alpha3.StringMatch_Exact{Exact: operation.Method},
					},
				},
			},
			Route: []*v1alpha3.HTTPRouteDestination{},
		}

		// Find the backendServer referenced by the operation.
		// TODO: Return an error if the reference is not found.
		for _, backendServer := range api.Spec.BackendServer {
			if backendServer.Name == operation.BackendServerName {
				httpRouteDestination := v1alpha3.HTTPRouteDestination{
					Destination: &v1alpha3.Destination{
						//TODO: Detect the cluster host and append it, instead of hardcoding it.
						Host: backendServer.ServiceRef.Name + "." + backendServer.ServiceRef.Namespace + ".svc." +
							"cluster.local",
					},
				}
				httpRoute.Route = append(httpRoute.Route, &httpRouteDestination)
			}
		}
		httpRoutes[i] = &httpRoute
	}
	return httpRoutes
}

func (is *IstioProvider) Validate(api v1beta1.API) error {
	return nil
}

func (is *IstioProvider) Update(ctx context.Context, api v1beta1.API) error {
	return nil
}

func (is *IstioProvider) Status(api v1beta1.API) (bool, error) {
	return true, nil
}

func (is *IstioProvider) Delete(ctx context.Context, api v1beta1.API) error {
	return nil
}
