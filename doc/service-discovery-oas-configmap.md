# Setup HTTP routing rules from OpenAPI stored in a configmap

This guide shows how to define the routing rules
from the [OpenAPI Specification (OAS) 3.x](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.2.md)
stored in a config map.

## Table of contents

* [Preparation](#preparation)
* [Create a ConfigMap with the OpenAPI document](#create-a-configmap-with-the-openapi-document)
* [Activate the service discovery](#activate-the-service-discovery)
* [Test the Toy Store API](#test-the-toy-store-api)
* [Next steps](#next-steps)

## Preparation

Follow [Getting Started](/doc/getting-started.md) to have the Toy Store service
being protected by kuadrant.


## Create a ConfigMap with the OpenAPI document

Have the following OpenAPI document in a file called `toystore.yaml`

```yaml
❯ cat toystore.yaml
---
openapi: "3.0.0"
info:
  title: "Toy Store"
  description: "The Toy Store OpenAPI"
  version: "1.0.0"
paths:
  /toy:
    get:
      operationId: "getToy"
      responses:
        405:
          description: "invalid input"
```

Create the config map

```bash
❯ kubectl -n default create configmap toystore --from-file=openapi.yaml=toystore.yaml
configmap/toystore created
```

## Activate the service discovery

In order to activate the service discovery, the upstream Toy Store API service needs to be labeled.

```bash
❯ kubectl -n default label service toystore discovery.kuadrant.io/enabled=true
service/toystore labeled
```

We need to add an annotation to the Toy Store service.
The annotation will have a reference to the recently created config map.

```bash
❯ kubectl -n default annotate service toystore discovery.kuadrant.io/oas-configmap=toystore
service/toystore annotated
```

Verify that the Toy Store kuadrant API object has been created with the OpenAPI document.

```bash
❯ kubectl -n default get api toystore -o yaml
apiVersion: networking.kuadrant.io/v1beta1
kind: API
metadata:
  name: toystore
  namespace: default
spec:
  destination:
    schema: http
    serviceReference:
      name: toystore
      namespace: default
      port: 80
  mappings:
    OAS: |
      ---
      openapi: "3.0.0"
      info:
        title: "Toy Store"
        description: "The Toy Store OpenAPI"
        version: "1.0.0"
      paths:
        /toy:
          get:
            operationId: "getToy"
            responses:
              405:
                description: "invalid input"
```

### Create kuadrant API Product object

The kuadrant API Product custom resource represents the kuadrant protection configuration for your service.
For this user guide, we will be creating the minimum configuration required to integrate kuadrant with your service.

```yaml
❯ cat apiproduct.yaml
---
apiVersion: networking.kuadrant.io/v1beta1
kind: APIProduct
metadata:
  name: toystore
  namespace: default
spec:
  hosts:
    - '*'
  APIs:
    - name: toystore
      namespace: default
```

```bash
❯ kubectl -n default apply -f apiproduct.yaml
apiproduct.networking.kuadrant.io/toystore created
```

Verify the APIProduct ready condition status is `true`

```bash
❯ kubectl get apiproduct toystore -n default -o jsonpath="{.status}" | jq '.'
{
  "conditions": [
    {
      "message": "Ready",
      "reason": "Ready",
      "status": "True",
      "type": "Ready"
    }
  ],
  "observedgen": 1
}
```

## Test the Toy Store API

Run kubectl port-forward in a different shell:

```bash
❯ kubectl port-forward -n kuadrant-system service/kuadrant-gateway 9080:80
Forwarding from [::1]:9080 -> 8080
```

The service be can now accessed at `http://localhost:9080` via a browser or any other client, like curl.

Requesting `GET /toy` should work:

```bash
❯ curl localhost:9080/toy
{
  "method": "GET",
  "path": "/toy",
  "query_string": null,
  "body": "",
  "headers": {
    "HTTP_HOST": "localhost:9080",
    "HTTP_USER_AGENT": "curl/7.68.0",
    "HTTP_ACCEPT": "*/*",
    "HTTP_X_FORWARDED_FOR": "10.244.0.1",
    ...
    "HTTP_X_B3_SAMPLED": "0",
    "HTTP_VERSION": "HTTP/1.1"
  },
  "uuid": "7425d080-c663-405f-a943-4df479a78dc7"
}
```

On the other hand, any other request should be rejected.

```bash
❯ curl -I -X POST localhost:9080/toy
HTTP/1.1 404 Not Found
date: Thu, 14 Oct 2021 16:03:18 GMT
server: istio-envoy
transfer-encoding: chunked


❯ curl -I localhost:9080/somethingelse
HTTP/1.1 404 Not Found
date: Thu, 14 Oct 2021 16:03:18 GMT
server: istio-envoy
transfer-encoding: chunked
```

## Next steps

Check out other [user guides](/README.md#user-guides) for other kuadrant capabilities like AuthN or rate limit.
