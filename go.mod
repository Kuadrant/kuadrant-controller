module github.com/kuadrant/kuadrant-controller

go 1.16

require (
	github.com/go-logr/logr v1.2.3
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/google/uuid v1.2.0
	github.com/kuadrant/authorino v0.10.0
	github.com/kuadrant/limitador-operator v0.3.1-0.20220830090346-4f6d5794272b
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	go.uber.org/zap v1.19.1
	gotest.tools v2.2.0+incompatible
	istio.io/api v0.0.0-20220516175159-89828b1f4baa
	istio.io/client-go v1.13.3
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/klog/v2 v2.60.1
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/gateway-api v0.4.1
)
