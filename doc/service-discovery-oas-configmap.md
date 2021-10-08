# Kuadrant Service Discovery

## Table of contents

TODO

Let's see this use case in action step by step.

**INSTALL KUADRANT**

Follow [kuadrant installation steps](/README.md#getting-started) to have kuadrant up and running.

**DEPLOY THE TOY STORE API APPLICATION**

Skip this section if you already have your service deployed.

The Toy Store API Service will be backed by a simple Echo API service.

```bash
❯ kubectl apply -n default -f https://raw.githubusercontent.com/kuadrant/kuadrant-controller/main/examples/toystore/toystore.yaml
```

Verify that the Toy Store pod is up and running.

```bash
❯ kubectl get pods -n default --field-selector=status.phase==Running
NAME                        READY   STATUS    RESTARTS   AGE
toystore-XXXXXXXXXX-XXXXX   1/1     Running   0          2m56s
```

Verify that the Toy Store service has been created.

```bash
❯ kubectl get service toystore -n default
NAME       TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
toystore   ClusterIP   10.XX.XXX.XXX   <none>        80/TCP    4m28s
```

**STORE THE OPENAPI IN A CONFIGMAP

**ACTIVATE SERVICE DISCOVERY**
