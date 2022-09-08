package apim

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kuadrant/kuadrant-controller/pkg/common"
)

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
