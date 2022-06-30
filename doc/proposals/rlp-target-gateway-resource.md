# RLP can target a Gateway resource

Previous version: https://hackmd.io/IKEYD6NrSzuGQG1nVhwbcw

Based on: https://hackmd.io/_1k6eLCNR2eb9RoSzOZetg

## Introduction

The [current RateLimitPolicy CRD](https://github.com/Kuadrant/kuadrant-controller/blob/fa2b52967409b7c4ea2c2e3412ecf80a8ad2b802/apis/apim/v1alpha1/ratelimitpolicy_types.go#L132) already implements a `targetRef` with a reference to [Gateway API's HTTPRoute](https://gateway-api.sigs.k8s.io/v1alpha2/references/spec/#gateway.networking.k8s.io/v1alpha2.HTTPRoute). This doc  captures the design and some implementation details of allowing the `targetRef` to reference a [Gateway API's Gateway](https://gateway-api.sigs.k8s.io/v1alpha2/references/spec/#gateway.networking.k8s.io/v1alpha2.Gateway).

Having in place this HTTPRoute - Gateway hierarchy, we are also considering to apply [Policy Attachment's](https://gateway-api.sigs.k8s.io/v1alpha2/references/policy-attachment/) defaults/overrides approach to the RateLimitPolicy CRD. But for now, it will only be about targeting the Gateway resource.

![](https://i.imgur.com/UkivAqA.png)

On designing kuadrant rate limiting and considering Istio/Envoy's rate limiting offering, we hit two limitations ([described here](https://docs.google.com/document/d/1ve_8ZBq8TK_wnAZHg69M6-f_q1w-mX4vuP1BC1EuEO8/edit#bookmark=id.5wyq2fj56u94)). Therefore, not giving up entirely in existing [Envoy's RateLimit Filter](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/ratelimit/v3/rate_limit.proto#extension-envoy-filters-network-ratelimit), we decided to move on and leverage the Envoy's [Wasm Network Filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/listeners/network_filters/wasm_filter) and implement rate limiting [wasm-shim](https://github.com/Kuadrant/wasm-shim) module compliant with the Envoy's [Rate Limit Service (RLS)](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/ratelimit/v3/rls.proto). This wasm-shim module accepts a [PluginConfig](https://github.com/Kuadrant/kuadrant-controller/blob/fa2b52967409b7c4ea2c2e3412ecf80a8ad2b802/pkg/istio/wasm.go#L24) struct object as input configuration object.

## Use Cases targeting a gateway
A key use case is being able to provide governance over what service providers can and cannot do when exposing a service via a shared ingress gateway. As well as providing certainty that no service is exposed without my ability as a cluster administrator to protect my infrastructure from unplanned load from badly behaving clients etc.

## Goals

The goal of this document is to define:
* The schema of this `PluginConfig` struct.
* The kuadrant-controller behavior filling the `PluginConfig` struct having as input the RateLimitPolicy k8s objects
* The behavior of the wasm-shim having the `PluginConfig` struct as input.

## Envoy's Rate Limit Service Potocol

Kuadrant's rate limit relies on the [Rate Limit Service (RLS)](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/ratelimit/v3/rls.proto) protocol, hence the gateway generates, based on a set of [actions](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route_components.proto#envoy-v3-api-msg-config-route-v3-ratelimit-action), a set of [descriptors](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/common/ratelimit/v3/ratelimit.proto#envoy-v3-api-msg-extensions-common-ratelimit-v3-ratelimitdescriptor) (one descriptor is a set of descriptor entries). Those descriptors are send to the external rate limit service provider. When multiple descriptors are provided, the external service provider will limit on ALL of them and return an OVER_LIMIT response if any of them are over limit.

## Schema (CRD) of the RateLimitPolicy

```yaml
---
apiVersion: apim.kuadrant.io/v1alpha1
kind: RateLimitPolicy
metadata:
  name: my-rate-limit-policy
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute / Gateway
    name: myroute / mygateway
  rateLimits:
    - operations:
        - paths: ["/admin/*"]
          methods: ["GET"]
          hosts: ["example.com"]
      actions:
        - generic_key:
            descriptor_key: admin
            descriptor_value: "yes"
      limits:
        - conditions: ["admin == yes"]
          max_value: 500
          seconds: 30
          variables: []
```

**Note:** No `namespace`/`domain` defined. Kuadrant controller will figure out.
**Note:** There is no `PREAUTH`, `POSTAUTH` stage defined. Ratelimiting filter should be placed after authorization filter to enable authenticated rate limiting. In the future, `stage` can be implemented.

## Kuadrant-controller's behavior

One HTTPRoute can only be targeted by one rate limit policy. 

Similarly, one Gateway can only be targeted by one rate limit policy.

However, indirectly, one gateway will be affected by multiple rate limit policies. 
It is by design of the Gateway API, one gateway can be referenced by multiple HTTPRoute objects. 
Furthermore, one HTTPRoute can reference multiple gateways.

The kuadrant controller will aggregate all the rate
limit policies that apply for each gateway, including RLP targeting HTTPRoutes and Gateways.

#### "VirtualHosting" RateLimitPolicies

Rate limit policies are scoped by the domains defined at the referenced HTTPRoute's
[hostnames](https://gateway-api.sigs.k8s.io/v1alpha2/references/spec/#gateway.networking.k8s.io/v1alpha2.HTTPRouteSpec)
and Gateway's Listener's [Hostname](https://gateway-api.sigs.k8s.io/v1alpha2/references/spec/#gateway.networking.k8s.io%2fv1alpha2.Listener).

#### Multiple HTTPRoutes with the same hostname

When there are multiple HTTPRoutes with the same hostname, HTTPRoutes are all admitted and
envoy merge the routing configuration in the same virtualhost. In these cases, the control plane
has to "merge" the rate limit configuration into a single entry for the wasm filter.

#### Overlapping HTTPRoutes

If some RLP targets a route for `*.com` and other RLP targets another route for `api.com`,
the control plane does not do any *merging*.
A request coming for `api.com` will be rate limited with the rules from the RLP targeting
the route `api.com`.
Also, a request coming for `other.com` will be rate limited with the rules from the RLP targeting
the route `*.com`.

### examples

RLP A -> HTTPRoute A (`api.toystore.com`) -> Gateway G (`*.com`)

RLP B -> HTTPRoute B (`other.toystore.com`) -> Gateway G (`*.com`)

RLP H -> HTTPRoute H (`*.toystore.com`) -> Gateway G (`*.com`)

RLP G -> Gateway G (`*.com`)

Request 1 (`api.toystore.com`) -> apply RLP A and RLP G

Request 2 (`other.toystore.com`) -> apply RLP B and RLP G

Request 3 (`unknown.toystore.com`) -> apply RLP H and RLP G

Request 4 (`other.com`) -> apply RLP G

#### rate limit domain / limitador namespace

The kuadrant controller will add `domain` attribute of the Envoy's [Rate Limit Service (RLS)](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/ratelimit/v3/rls.proto). It will also add the `namespace` attribute of the Limitador's rate limit config. The controller will ensure that the associated actions and rate limits have a common domain/namespace.

The value of this domain/namespace seems to be related to the virtualhost for which rate limit applies.

## Schema of the `PluginConfig` struct

Currently the PluginConfig looks like this:

```yaml
#  The filter’s behaviour in case the rate limiting service does not respond back. When it is set to true, Envoy will not allow traffic in case of communication failure between rate limiting service and the proxy.
failure_mode_deny: true
ratelimitpolicies:
  default/toystore: # rate limit policy {NAMESPACE/NAME}
    hosts: # HTTPRoute hostnames
      - '*.toystore.com'
    rules: # route level actions
      - operations:
          - paths:
              - /admin/toy
            methods:
              - POST
              - DELETE
        actions:
          - generic_key:
              descriptor_value: yes
              descriptor_key: admin
    global_actions: # virtualHost level actions
      - generic_key:
          descriptor_value: yes
          descriptor_key: vhaction
    upstream_cluster: rate-limit-cluster # Limitador address reference
    domain: toystore-app # RLS protocol domain value
```
Proposed new design:

```yaml
#  The filter’s behaviour in case the rate limiting service does not respond back. When it is set to true, Envoy will not allow traffic in case of communication failure between rate limiting service and the proxy.
failureModeDeny: true
rate_limit_policies:
  - name: toystore
    rate_limit_domain: toystore-app
    upstream_cluster: rate-limit-cluster
    hostnames: ["*.toystore.com"]
    gateway_actions:
      - rules:
          - paths: ["/admin/toy"]
            methods: ["GET"]
            hosts: ["pets.toystore.com"]
        configurations:
          - actions:
            - generic_key:
                descriptor_key: admin
                descriptor_value: "1"
```

Update highlights:
* [*minor*] `rate_limit_policies` is a list instead of a map indexed by the name/namespace.
* [*major*] no distinction between "rules" and global actions
* [*major*] more aligned with RLS: [multiple descriptors](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route_components.proto#envoy-v3-api-field-config-route-v3-virtualhost-rate-limits) structured by "rate limit configurations" with matching rules

## WASM-SHIM

Each rate limit policy has a logical name and a set of hostnames to activate it based on the incoming request’s host header.

The WASM-SHIM builds a tree based data structure holding the rate limit policies.
The longest (sub)domain match is used to select the policy to be applied.
Only one policy is being applied per invocation.

The plugin object contains a list of configurations to build a list of Envoy's RLS descriptors.
These policy configurations are defined at

```
rate_limit_policies[*].gatewat_actions[*].configurations
```

For example:

```yaml
configurations:
- actions:
   - generic_key:
        descriptor_key: admin
        descriptor_value: "1"
```

Each configuration produces, at most, one descriptor. Depending on the incoming request, one configuration may or may not produce a rate limit descriptor.

Each policy configuration has associated, optionally, a set of rules to match. Rules allow matching `hosts` and/or `methods` and/or `paths`. Matching occurs when at least one rule applies against the incoming request. If rules are not set, it is equivalent to matching all the requests.

For example:

```yaml
gateway_actions:
      - rules:
          - paths: ["/admin/toy"]
            methods: ["GET"]
            hosts: ["pets.toystore.com"]
        configurations:
          - actions:
            - generic_key:
                descriptor_key: admin
                descriptor_value: "1"
```

Each configuration object defines a list of actions.
Each action may (or may not) produce a descriptor entry (descriptor list item).
If an action cannot append a descriptor entry, no descriptor is generated for the configuration.

The external rate limit service will be called when there is at least one not empty descriptor
generated by the list of configurations.
