/*
Copyright 2024.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
)

var _ = Describe("Alias Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		alias := &operatorv1alpha1.Alias{}

		mailuMock := mailuMock()

		BeforeEach(func() {
			By("creating the custom resource for the Kind Alias")
			err := k8sClient.Get(ctx, typeNamespacedName, alias)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.Alias{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: operatorv1alpha1.AliasSpec{
						Name:   "foo",
						Domain: "example.com",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &operatorv1alpha1.Alias{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Alias")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AliasReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   mailuMock,
				ApiToken: "asdf",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// reconcile again to cover "update" path
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			alias := &operatorv1alpha1.Alias{}
			err = k8sClient.Get(ctx, typeNamespacedName, alias)
			Expect(err).NotTo(HaveOccurred())
			Expect(alias.Status.Conditions).ToNot(BeEmpty())
		})
	})
})