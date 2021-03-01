module github.com/kuadrant/kuadrant-controller

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	istio.io/api v0.0.0-20210219142745-68975986cccb
	istio.io/client-go v1.9.0
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/gateway-api v0.2.0 // indirect
)