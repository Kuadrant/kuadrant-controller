# Basic setup

This guide lets you quickly integrate kuadrant with your service with the minimal configuration needed.

## Table of contents

TODO

## Steps

### Install kuadrant

Follow [kuadrant installation steps](/README.md#getting-started) to have kuadrant up and running.

### Deploy the upstream Toy Store API service

Skip this section if you already have your service deployed.

The Toy Store API Service will be backed by a simple Echo API service.

```bash
❯ kubectl apply -n default -f https://raw.githubusercontent.com/kuadrant/kuadrant-controller/main/examples/toystore/toystore.yaml
```

Verify that the Toy Store pod is up and running.

```bash
❯ kubectl get pods --field-selector=status.phase==Running
NAME                        READY   STATUS    RESTARTS   AGE
toystore-XXXXXXXXXX-XXXXX   1/1     Running   0          2m56s
```

Verify that the Toy Store service has been created.

```bash
❯ kubectl get service toystore -n default
NAME       TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
toystore   ClusterIP   10.XX.XXX.XXX   <none>        80/TCP    4m28s
```

### Create kuadrant API object

The kuadrant API custom resource represents a specific kubernetes service.
It needs to be created for each service that is wanted to be protected by kuadrant.

There are two methods to make it easy for you to create kuadrant API objects:
* [kuadrantctl CLI](https://github.com/Kuadrant/kuadrantctl/blob/main/doc/api-apply.md) tool with the following command `kuadrantctl api apply --service-name <SERVICE>`
* The [kuadrant service discovery](managing-apis.md#service-discovery) system watches for services labeled with kuadrant

For this user guide, we will be using the [kuadrant service discovery](managing-apis.md#service-discovery).
To activate it, the upstream Toy Store API service needs to be labeled.

```bash
❯ kubectl -n default label service toystore discovery.kuadrant.io/enabled=true
service/toystore labeled
```

Verify that the Toy Store kuadrant API object has been created.

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
    HTTPPathMatch:
      type: Prefix
      value: /
```

*Note*: some kubernetes specific data has been removed from the snippet above just for clarity.

### Create kuadrant API Product object

The kuadrant API Product custom resource represents the kuadrant protection configuration for your service.
For this user guide, we will be creating the minimum configuration required to integrate kuadrant with your service.

```yaml
---
apiVersion: networking.kuadrant.io/v1beta1
kind: APIProduct
metadata:
  name: toystore
spec:
  routing:
    hosts:
      - '*'
  APIs:
    - name: toystore
      namespace: default
```

### Verify the kuadrant service
