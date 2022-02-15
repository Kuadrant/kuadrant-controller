package apim

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	apimv1alpha1 "github.com/kuadrant/kuadrant-controller/apis/apim/v1alpha1"
	"github.com/kuadrant/kuadrant-controller/pkg/common"
	istio "istio.io/client-go/pkg/apis/networking/v1alpha3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	patchesFinalizer = "kuadrant.io/ratelimitpatches"

	ParentRefsSeparator = ","

	EnvoyFilterParentRefsAnnotation = "kuadrant.io/parentRefs"
)

// finalizeEnvoyFilters makes sure orphan EnvoyFilter resources are not left when deleting the owner RateLimitPolicy.
func (r *RateLimitPolicyReconciler) finalizeEnvoyFilters(ctx context.Context, rlp *apimv1alpha1.RateLimitPolicy) error {
	logger := logr.FromContext(ctx)
	logger.Info("Removing/Updating EnvoyFilter resources")
	ownerRlp := client.ObjectKeyFromObject(rlp).String()

	// TODO(rahulanand16nov): do the same for HTTPRoute
	vsList := &istio.VirtualServiceList{}
	if err := r.Client().List(ctx, vsList, &client.ListOptions{Namespace: rlp.Namespace}); err != nil {
		logger.Error(err, "failed to list VirtualServices in RateLimitPolicy's namespace")
		return err
	}

	for _, vs := range vsList.Items {
		if val, present := vs.Annotations[KuadrantRateLimitPolicyAnnotation]; !present || (val != rlp.Name) {
			continue
		}
		vsKey := client.ObjectKeyFromObject(&vs)
		for _, gateway := range vs.Spec.Gateways {
			gwKey := common.NamespacedNameToObjectKey(gateway, vs.Namespace)

			filtersPatchkey := client.ObjectKey{
				Namespace: gwKey.Namespace,
				Name:      rlFiltersPatchName(gwKey.Name),
			}
			filtersPatch := &istio.EnvoyFilter{}
			if err := r.Client().Get(ctx, filtersPatchkey, filtersPatch); err != nil {
				if !apierrors.IsNotFound(err) {
					logger.Error(err, "failed to get ratelimits filters patch")
					return err
				}
				logger.V(1).Info("filters patch not found", "object key", filtersPatchkey.String())
			}
			if err := r.removeParentRefEntry(ctx, filtersPatch, ownerRlp); err != nil {
				logger.Error(err, "failed to remove parentRef on filters patch")
				return err
			}

			logger.Info("successfully removed parentRef entry on the filters patch")

			ratelimitsPatchKey := client.ObjectKey{
				Namespace: gwKey.Namespace,
				Name:      ratelimitsPatchName(gwKey.Name, vsKey),
			}
			ratelimitsPatch := &istio.EnvoyFilter{}
			if err := r.Client().Get(ctx, ratelimitsPatchKey, ratelimitsPatch); err != nil {
				if apierrors.IsNotFound(err) {
					logger.Error(err, "failed to get ratelimits patch")
					return err
				}
				logger.V(1).Info("ratelimits patch not found", "object key", ratelimitsPatchKey.String())
			}
			if err := r.removeParentRefEntry(ctx, ratelimitsPatch, ownerRlp); err != nil {
				logger.Error(err, "failed to remove remove parentRef entry on ratelimits patch")
				return err
			}
			logger.Info("successfully removed parentRef tag on ratelimits patch")
		}
	}
	return nil
}

func (r *RateLimitPolicyReconciler) removeParentRefEntry(ctx context.Context, patch *istio.EnvoyFilter, owner string) error {
	logger := logr.FromContext(ctx)
	logger.Info("Removing parentRef entry from EnvoyFilter", "EnvoyFilter", patch.Name)

	// find the annotation
	ownerRlpsVal, present := patch.Annotations[EnvoyFilterParentRefsAnnotation]
	if !present {
		logger.V(1).Info("Deleting the patch since parentRef annotation was not present to avoid orphans")
		// if it's not deleted then the patch will remain as an orphan once all the rlps are removed.
		if err := r.Client().Delete(ctx, patch); err != nil {
			logger.Error(err, "failed to delete the patch")
			return err
		}
		return nil
	}

	// split into array of RateLimitPolicy names
	ownerRlps := strings.Split(ownerRlpsVal, ParentRefsSeparator)

	// remove the target owner
	finalOwnerRlps := []string{}
	for idx := range ownerRlps {
		if ownerRlps[idx] == owner {
			continue
		}
		finalOwnerRlps = append(finalOwnerRlps, ownerRlps[idx])
	}

	if len(finalOwnerRlps) == 0 {
		logger.V(1).Info("Deleting filters patch because 0 parentRef entries found on it")
		if err := r.Client().Delete(ctx, patch); err != nil {
			logger.Error(err, "failed to delete the patch")
			return err
		}
	} else {
		finalOwnerRlpsVal := strings.Join(finalOwnerRlps, ParentRefsSeparator)
		patch.Annotations[EnvoyFilterParentRefsAnnotation] = finalOwnerRlpsVal
		if err := r.Client().Update(ctx, patch); err != nil {
			logger.Error(err, "failed to update the patch")
			return err
		}
	}
	return nil
}

func (r *RateLimitPolicyReconciler) addParentRefEntry(ctx context.Context, patch *istio.EnvoyFilter, owner string) error {
	logger := logr.FromContext(ctx)
	logger.Info("Adding parentRef entry to EnvoyFilter", "EnvoyFilter", patch.Name)

	// make sure annotation map is initialized
	patchOwnerList := []string{}
	if patch.Annotations == nil {
		patch.Annotations = make(map[string]string)
	}

	if ogOwnerRlps, present := patch.Annotations[EnvoyFilterParentRefsAnnotation]; present {
		patchOwnerList = strings.Split(ogOwnerRlps, ParentRefsSeparator)
	}

	// add the owner name if not present already
	if !common.Contains(patchOwnerList, owner) {
		patchOwnerList = append(patchOwnerList, owner)
	}
	finalOwnerVal := strings.Join(patchOwnerList, ParentRefsSeparator)

	patch.Annotations[EnvoyFilterParentRefsAnnotation] = finalOwnerVal
	if err := r.ReconcileResource(ctx, &istio.EnvoyFilter{}, patch, alwaysUpdateEnvoyPatches); err != nil {
		logger.Error(err, "failed to create/update EnvoyFilter that patches-in ratelimit filters")
		return err
	}

	logger.Info("Successfully added parentRef entry to the EnvoyFilter")
	return nil
}
