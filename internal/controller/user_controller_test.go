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

	operatorv1alpha1 "gitlab.rootcrew.net/rootcrew/services/mailu-operator/api/v1alpha1"
)

var _ = Describe("User Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		user := &operatorv1alpha1.User{}

		mailuMock := mailuMock()

		BeforeEach(func() {
			By("creating the custom resource for the Kind User")
			err := k8sClient.Get(ctx, typeNamespacedName, user)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
					Spec: operatorv1alpha1.UserSpec{
						Name:   "foo",
						Domain: "example.com",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &operatorv1alpha1.User{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance User")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &UserReconciler{
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

			user := &operatorv1alpha1.User{}
			err = k8sClient.Get(ctx, typeNamespacedName, user)
			Expect(err).NotTo(HaveOccurred())
			Expect(user.Status.Conditions).ToNot(BeEmpty())
		})

		It("should successfully remove the the resource", func() {
			By("Reconciling the deleted resource")
			err := k8sClient.Delete(ctx, user)
			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &UserReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   mailuMock,
				ApiToken: "asdf",
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			user := &operatorv1alpha1.User{}
			err = k8sClient.Get(ctx, typeNamespacedName, user)
			Expect(err).To(HaveOccurred())
		})

		It("should fail on invalid resource spec", func() {
			By("Preventing the resource from being created")
			// TODO: find a better way to create a different resource via JustBeforeEach?
			controllerReconciler := &UserReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   mailuMock,
				ApiToken: "asdf",
			}

			err := k8sClient.Delete(ctx, user)
			Expect(err).NotTo(HaveOccurred())

			// reconcile to finish deletion of the resource
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// try creating the resource with invalid date
			resource := &operatorv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: operatorv1alpha1.UserSpec{
					Name:           "foo",
					Domain:         "example.com",
					ReplyStartDate: "1900-01-31",
					ReplyEndDate:   "0000-00-00",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).NotTo(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			user := &operatorv1alpha1.User{}
			err = k8sClient.Get(ctx, typeNamespacedName, user)
			Expect(err).To(HaveOccurred())
		})
	})
})
