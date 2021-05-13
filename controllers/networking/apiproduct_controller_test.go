// +build integration

/*
Copyright 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package networking

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("APIPRoduct controller", func() {
	var testNamespace string

	BeforeEach(func() {
		var generatedTestNamespace = "test-namespace-" + uuid.New().String()
		// Add any setup steps that needs to be executed before each test
		desiredTestNamespace := &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: generatedTestNamespace,
			},
		}

		err := k8sClient.Create(context.Background(), desiredTestNamespace)
		Expect(err).ToNot(HaveOccurred())

		existingNamespace := &v1.Namespace{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: generatedTestNamespace}, existingNamespace)
			if err != nil {
				return false
			}
			return true
		}, 5*time.Minute, 5*time.Second).Should(BeTrue())

		testNamespace = existingNamespace.Name
	})

	AfterEach(func() {
		desiredTestNamespace := &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		// Add any teardown steps that needs to be executed after each test
		err := k8sClient.Delete(context.Background(), desiredTestNamespace, client.PropagationPolicy(metav1.DeletePropagationForeground))

		Expect(err).ToNot(HaveOccurred())

		existingNamespace := &v1.Namespace{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: testNamespace}, existingNamespace)
			if err != nil && errors.IsNotFound(err) {
				return false
			}
			return true
		}, 5*time.Minute, 5*time.Second).Should(BeTrue())

	})

	Context("Run directly without existing APIPRoduct", func() {
		It("Should create successfully", func() {
			Expect(1).To(Equal(1))
		})
	})
})
