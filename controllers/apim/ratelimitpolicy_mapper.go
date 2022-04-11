package apim

import (
	"github.com/go-logr/logr"
	"github.com/kuadrant/kuadrant-controller/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// TODO(eastizle): KuadrantAddVSAnnotation annotation does not support multiple VirtualServices having reference to the same RateLimitPolicy
	// These annotations are put on RateLimitPolicy resource to signal network change.
	// Note: the annotation key is fixed, the RLP name is in the value
	KuadrantAddVSAnnotation    = "kuadrant.io/attach-virtualservice"
	KuadrantDeleteVSAnnotation = "kuadrant.io/detach-virtualservice"
	KuadrantAddHRAnnotation    = "kuadrant.io/attach-httproute"
	KuadrantDeleteHRAnnotation = "kuadrant.io/detach-httproute"

	// These annotations help reconcilers know which signal to send to the RateLimitPolicy.
	KuadrantAttachNetwork = "kuadrant.io/attach-network"
	KuadrantDetachNetwork = "kuadrant.io/detach-network"
)

// TODO(rahulanand16nov): separate auth and ratelimit (single responsibility principle)
// routingPredicate is used by routing objects' controllers to filter for Kuadrant annotations signaling API protection.
func RoutingPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, toProtect := e.Object.GetAnnotations()[common.KuadrantAuthProviderAnnotation]
			return toProtect
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, toProtectOld := e.ObjectOld.GetAnnotations()[common.KuadrantAuthProviderAnnotation]
			_, toProtectNew := e.ObjectNew.GetAnnotations()[common.KuadrantAuthProviderAnnotation]
			return toProtectOld || toProtectNew
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, toProtect := e.Object.GetAnnotations()[common.KuadrantAuthProviderAnnotation]
			return toProtect
		},
	}
}

// HTTPRouteEventMapper is an EventHandler that maps HTTPRoute object events
// to RateLimitPolicy events.
type HTTPRouteEventMapper struct {
	Logger logr.Logger
}

func (h *HTTPRouteEventMapper) Map(obj client.Object) []reconcile.Request {
	httpRouteAnnotations := obj.GetAnnotations()
	if httpRouteAnnotations == nil {
		httpRouteAnnotations = map[string]string{}
	}

	rateLimitRef, ok := httpRouteAnnotations[common.RateLimitPolicyBackRefAnnotation]
	if !ok {
		return []reconcile.Request{}
	}

	rlpKey := common.NamespacedNameToObjectKey(rateLimitRef, obj.GetNamespace())
	h.Logger.V(1).Info("Processing object", "key", client.ObjectKeyFromObject(obj), "ratelimitpolicy", rlpKey)

	requests := []reconcile.Request{
		{
			NamespacedName: rlpKey,
		},
	}
	return requests
}
