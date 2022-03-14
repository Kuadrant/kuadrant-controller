### Installing Istio in openshift

[Official doc: Istio on Openshift](https://istio.io/latest/docs/setup/platform-setup/openshift/)

```
oc adm policy add-scc-to-group anyuid system:serviceaccounts:istio-system
```

```
istioctl install --set profile=openshift
```

### Deploy Toystore app

```
oc new-project myns
oc adm policy add-scc-to-group anyuid system:serviceaccounts:myns
oc apply -f examples/toystore/toystore.yaml
```

### Inject sidecar

```
kubectl patch deployment toystore --patch '{"spec": {"template": {"metadata": {"labels": {"sidecar.istio.io/inject": "true"}}}}}'
```

### Install kuadrant

Patch istio with Authorino as ext authZ
```
kubectl edit configmap istio -n istio-system
```

In the editor, add the extension provider definitions
```yaml
data:
  mesh: |-
    # Add the following content to define the external authorizers.
    extensionProviders:
    - name: "kuadrant-authorization"
      envoyExtAuthzGrpc:
        service: "authorino-authorino-authorization.kuadrant-system.svc.cluster.local"
        port: "50051"
```

Restart Istiod to allow the change to take effect with the following command:
```
kubectl rollout restart deployment/istiod -n istio-system
```

Install kuadrant components

```
export KUADRANT_NAMESPACE="kuadrant-system"
oc new-project "${KUADRANT_NAMESPACE}"

# Authorino
kubectl apply -f utils/local-deployment/authorino-operator.yaml
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/authorino.yaml

# Limitador
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/limitador-operator.yaml
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/limitador.yaml

kubectl -n "${KUADRANT_NAMESPACE}" wait --timeout=300s --for=condition=Available deployments --all

make run
```

### User Guide
