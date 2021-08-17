#!/bin/bash

#
# Copyright 2021 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -euo pipefail

export KUADRANT_NAMESPACE="kuadrant-system"

echo "Creating namespace"
kubectl create namespace "${KUADRANT_NAMESPACE}"

echo "Deploying Ingress Provider"
kubectl apply -f utils/local-deployment/istio-manifests/Base/Base.yaml
kubectl apply -f utils/local-deployment/istio-manifests/Base/Pilot/Pilot.yaml
kubectl apply -f utils/local-deployment/istio-manifests/Base/Pilot/IngressGateways/IngressGateways.yaml
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/istio-manifests/default-gateway.yaml

echo "Deploying Authorino to the kuadrant-system namespace"
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/authorino.yaml

echo "Deploying Limitador Operator to the kuadrant-system namespace"
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/limitador-operator.yaml
kubectl apply -n "${KUADRANT_NAMESPACE}" -f utils/local-deployment/limitador.yaml

echo "Wait for all deployments to be up"
kubectl -n "${KUADRANT_NAMESPACE}" wait --timeout=300s --for=condition=Available deployments --all
