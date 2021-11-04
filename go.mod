module github.com/kuadrant/kuadrant-controller

go 1.16

require (
	github.com/Azure/go-autorest/autorest v0.11.19 // indirect
	github.com/getkin/kin-openapi v0.63.0
	github.com/go-logr/logr v0.4.0
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.1.2
	github.com/jarcoal/httpmock v1.0.8
	github.com/kuadrant/authorino v0.4.0
	github.com/kuadrant/limitador-operator v0.2.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	go.uber.org/zap v1.18.1
	gotest.tools v2.2.0+incompatible
	istio.io/api v0.0.0-20211119221920-e0ac4ca57eb8
	istio.io/client-go v1.12.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/controller-runtime v0.9.6
	sigs.k8s.io/gateway-api v0.4.0
)
