# Overview
This guide will walk you through setting up Kuadrant locally along side a Gateway api implementation provided by Istio and leveraging the Istio WASMPlugin resource to apply API Management features.

# Gateway API

[Gateway API](https://gateway-api.sigs.k8s.io/) is a standard API being developed to provide a set if resources for modeling  service networking

Istio has an implementation of this API and so can provide the infrastructure needed to handle ingress via the Gateway API resources. For this walkthrough we will be leveraging Istio.

Kuadrant can layer API Mgmt capabilities on top of the Gateway API by levaraging an Istio extension point (WASMPlugins).


# Trying it out


## Installation
You can try this out only a local cluster currently. You will need [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) installed as a prerequisite. 

```
make local-setup PROVIDER=gatewayapi
```
This command will provision Istio and Kuadrant along with the CRDs to support the Gateway API. It will also configure an initial gateway handled by Istio to support ingress. The host is for this gateway is configured as `*.example.com`

```
kubectl get deployments -n kuadrant-system 
```

Will show you all of the infrastructure components deployed.

## Enable Istio Sidecars

WASMPlugins are used to enable the API Management features. To apply these plugins to specific workloads, the WASMPlugins are injected within an Istio sidecar  To enable the injection of the sidecar, we must label our namespace with an Istio specific label.

```
kubectl label namespace default istio-injection=enabled
```

## Deploy a set of services

To represent out APIS we will deploy a simple set of services that represent APIs but are just echo APIs underneath.

```
kubectl create -f examples/gatewayapi/bookstore.yaml
kubectl create -f examples/gatewayapi/carstore.yaml
```

Within these objects we have added the following annotation
where <service> is replaced by books or cars
```
  annotations:
    discovery.kuadrant.io/matchpath: "/api/v1/<service>" 
  labels:
    discovery.kuadrant.io/enabled: "true"        
```   

We also add the label to enable Kuadrant to pick up the services and setup our API objects. To see the API objects run:
```
kubectl get apis -n default

```

## Create an API Product

To expose our API we will need create an APIProduct that brings together our two new two APIs into a single shop product.

```
kubectl create -f examples/gatewayapi/apiproduct-no-security.yaml
```

This APIProduct points at the APIs we want to package into a single product, defines which gateway we wish to use and also the host name for our API.

You should now be able to see a new HTTPRoute setup based on our service and APIProduct that has a host of shop.example.com

```
kubectl get httproute -n default

```

## Port forward the gateway to your local host

In order to make a request we need to port forward from our local machine to our gateway running on the kubernetes cluster.

```
k port-forward service/gateway-api 9080:80 -n kuadrant-system

```

## Test our API

We can now send some requests to our new API:

curl -v -HHost:shop.example.com "http://localhost:9080/api/v1/cars"

curl -v -HHost:shop.example.com "http://localhost:9080/api/v1/books"

should both return with 200 showing the headers etc from the echo API

curl -v -HHost:shop.example.com "http://localhost:9080/api/v1/pets"

should return with a 404


## Secure our API 

Next we will add some basic security to our API. To do this we will create a new secret with our defined API Key and update the APIProduct with a new security section.

```
kubectl create -f examples/gatewayapi/apikeysecret.yaml
```

This secret is labelled to allow authorino to pick it up and with a label to identify it to the APIProduct
```
    secret.kuadrant.io/managed-by: authorino
    api: store
```    

Now we will update the APIProduct with a security section

```
kubectl apply -f examples/gatewayapi/apiproduct-secured.yaml
```

This change add a security section to the APIProduct:
```
  securityScheme:
    - name: MyAPIKey
      apiKeyAuth:
        location: authorization_header
        name: APIKEY
        credential_source:
          labelSelectors:
            secret.kuadrant.io/managed-by: authorino
            api: store    
```

## Test our API

```
curl -v -HHost:shop.example.com "http://localhost:9080/api/v1/cars"

curl -v -HHost:shop.example.com "http://localhost:9080/api/v1/books"
```
should return 403 status codes now


Finally we will add our Authorization header based on the secret we created earlier

```
curl -v -H "Authorization: APIKEY JUSTFORDEMO" -HHost:shop.example.com "http://localhost:9080/api/v1/books"
curl -v -H "Authorization: APIKEY JUSTFORDEMO" -HHost:shop.example.com "http://localhost:9080/api/v1/cars"
```

We should one again get 200 responses and the contents of the echo API.
