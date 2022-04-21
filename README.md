# Kuadrant Controller

[![Code Style](https://github.com/Kuadrant/kuadrant-controller/actions/workflows/code-style.yaml/badge.svg)](https://github.com/Kuadrant/kuadrant-controller/actions/workflows/code-style.yaml)
[![Testing](https://github.com/Kuadrant/kuadrant-controller/actions/workflows/testing.yaml/badge.svg)](https://github.com/Kuadrant/kuadrant-controller/actions/workflows/testing.yaml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0)

## Table of contents

* [Overview](#overview)
* [CustomResourceDefinitions](#customresourcedefinitions)
* [Getting started](#getting-started)
* [Contributing](#contributing)
* [Licensing](#licensing)

## Overview

Kuadrant is a re-architecture of API Management using Cloud Native concepts and separating the components to be less coupled,
more reusable and leverage the underlying kubernetes platform. It aims to deliver a smooth experience to providers and consumers
of applications & services when it comes to rate limiting, authentication, authorization, discoverability, change management, usage contracts, insights, etc.

Kuadrant aims to produce a set of loosely coupled functionalities built directly on top of Kubernetes.
Furthermore it only strives to provide what Kubernetes doesn’t offer out of the box, i.e. Kuadrant won’t be designing a new Gateway/proxy,
instead it will opt to connect with what’s there and what’s being developed (think Envoy, GatewayAPI).

Kuadrant is a system of cloud-native k8s components that grows as users’ needs grow.
* From simple protection of a Service (via **AuthN**) that is used by teammates working on the same cluster, or “sibling” services, up to **AuthN** of users using OIDC plus custom policies.
* From no rate-limiting to rate-limiting for global service protection on to rate-limiting by users/plans

towards a full system that is more analogous to current API Management systems where business rules
and plans define protections and Business/User related Analytics are available.

## CustomResourceDefinitions

A core feature of the kuadrant controller is to monitor the Kubernetes API server for changes to
specific objects and ensure the owned k8s components configuration match these objects.
The kuadrant controller acts on the following [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/):

| CRD | Description |
| --- | --- |
| [RateLimitPolicy](apis/apim/v1alpha1/ratelimitpolicy_types.go) | Enable access control on workloads based on HTTP rate limiting |

For a detailed description of the CRDs above, refer to the [Architecture](doc/architecture.md) page.

## Getting started

1.- Clone Kuadrant controller and checkout main

```
git clone https://github.com/Kuadrant/kuadrant-controller
```

2.- Create local cluster and deploy kuadrant

```
make local-setup
```

3.- Deploy toystore example deployment

```
kubectl apply -f examples/toystore/toystore.yaml
```

4.- Create HTTPRoute to configure routing to the toystore service

```
kubectl apply -f - <<EOF
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
  name: toystore
  labels:
    app: toystore
spec:
  parentRefs:
    - name: kuadrant-gwapi-gateway
      namespace: kuadrant-system
  hostnames: ["*.toystore.com"]
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: "/toy"
          method: GET
        - path:
            type: Exact
            value: "/admin/toy"
          method: POST
        - path:
            type: Exact
            value: "/admin/toy"
          method: DELETE
      backendRefs:
        - name: toystore
          port: 80

EOF
```

Verify that we can reach our example deployment

```
curl -v -H 'Host: api.toystore.com' http://localhost:9080/toy
```

5.- Create RateLimitPolicy for ratelimiting

```
kubectl apply -f - <<EOF
---
apiVersion: apim.kuadrant.io/v1alpha1
kind: RateLimitPolicy
metadata:
  name: toystore
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: toystore
  rules:
    - operations:
        - paths: ["/toy"]
          methods: ["GET"]
      rateLimits:
        - stage: PREAUTH
          actions:
            - generic_key:
                descriptor_key: get-toy
                descriptor_value: "yes"
    - operations:
        - paths: ["/admin/toy"]
          methods: ["POST", "DELETE"]
      rateLimits:
        - stage: POSTAUTH
          actions:
            - generic_key:
                descriptor_key: admin
                descriptor_value: "yes"
            - metadata:
                descriptor_key: "user-id"
                default_value: "no-user"
                metadata_key:
                  key: "envoy.filters.http.ext_authz"
                  path:
                    - key: "ext_auth_data"
                    - key: "user-id"
  rateLimits:
    - stage: BOTH
      actions:
        - generic_key:
            descriptor_key: vhaction
            descriptor_value: "yes"
  domain: toystore-app
  limits:
    - conditions: ["get-toy == yes"]
      max_value: 2
      namespace: toystore-app
      seconds: 30
      variables: []
    - conditions:
      - "admin == yes"
      - "user-id == bob"
      max_value: 2
      namespace: toystore-app
      seconds: 30
      variables: []
    - conditions:
      - "addmin == yes"
      - "user-id == alice"
      max_value: 4
      namespace: toystore-app
      seconds: 30
      variables: []
    - conditions: ["vhaction == yes"]
      max_value: 6
      namespace: toystore-app
      seconds: 30
      variables: []
EOF
```

To verify wasm envoyfilter has been created:

```
kubectl get envoyfilter -A
NAMESPACE         NAME                                              AGE
kuadrant-system   kuadrant-kuadrant-gwapi-gateway-wasm-ratelimits   3m4s
```

To verify Limitador's RateLimit resources have been created:

```
kubectl get ratelimit -A
NAMESPACE         NAME                     AGE
kuadrant-system   rlp-default-toystore-1   49s
kuadrant-system   rlp-default-toystore-2   49s
kuadrant-system   rlp-default-toystore-3   49s
kuadrant-system   rlp-default-toystore-4   49s
```

6.- Verify unauthenticated rate limit

Only 2 requests every 30 secs on `GET /toy` operation allowed.

```
curl -v -H 'Host: api.toystore.com' http://localhost:9080/toy
```

8.- Add authentication

Create AuthConfig for Authorino external authz provider

```
kubectl apply -f - <<EOF
---
apiVersion: authorino.kuadrant.io/v1beta1
kind: AuthConfig
metadata:
  creationTimestamp: null
  name: toystore
spec:
  hosts:
    - api.toystore.com
  identity:
    - apiKey:
        labelSelectors:
          app: toystore
          authorino.kuadrant.io/managed-by: authorino
      credentials:
        in: authorization_header
        keySelector: APIKEY
      name: apikey
  response:
    - json:
        properties:
          - name: user-id
            value: null
            valueFrom:
              authJSON: auth.identity.metadata.annotations.secret\.kuadrant\.io/user-id
      name: rate-limit-apikey
      wrapper: envoyDynamicMetadata
      wrapperKey: ext_auth_data
EOF
```

Create secret with API key for user `bob`

```
kubectl apply -f examples/toystore/bob-api-key-secret.yaml
```

Create secret with API key for user `alice`

```
kubectl apply -f examples/toystore/alice-api-key-secret.yaml
```

Annotate HTTPRoute with Kuadrant auth provider to create AuthorizationPolicy

```
kubectl annotate httproute/toystore kuadrant.io/auth-provider=kuadrant-authorization
```

To verify creation:

```
kubectl get authorizationpolicy -A
NAMESPACE         NAME                                          AGE
kuadrant-system   on-kuadrant-gwapi-gateway-using-hr-toystore   3m36s
```

9.- Verify authentication

Should return `401 Unauthorized`

```
curl -v -H 'Host: api.toystore.com' -X POST http://localhost:9080/admin/toy
```

Should return `200 OK`

```
curl -v -H 'Host: api.toystore.com' -H 'Authorization: APIKEY ALICEKEYFORDEMO' -X POST http://localhost:9080/admin/toy
```

10.- Verify authenticated rate limit per user

4 times and should be rate limited

```
curl -v -H 'Host: api.toystore.com' -H 'Authorization: APIKEY ALICEKEYFORDEMO' -X POST http://localhost:9080/admin/toy
```

2 times and should be rate limited

```
curl -v -H 'Host: api.toystore.com' -H 'Authorization: APIKEY BOBKEYFORDEMO' -X POST http://localhost:9080/admin/toy
```

## Contributing

The [Development guide](doc/development.md) describes how to build the kuadrant controller and
how to test your changes before submitting a patch or opening a PR.

## Licensing

This software is licensed under the [Apache 2.0 license](https://www.apache.org/licenses/LICENSE-2.0).

See the LICENSE and NOTICE files that should have been provided along with this software for details.
