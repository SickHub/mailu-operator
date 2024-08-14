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
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
)

var _ = Describe("Domain Controller", func() {
	// TODO: unify context, use multiple "It" : https://onsi.github.io/ginkgo/
	Context("When reconciling a created resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		domain := &operatorv1alpha1.Domain{}

		mailuMock := mailuMock()

		BeforeEach(func() {
			By("creating the custom resource for the Kind Domain")
			err := k8sClient.Get(ctx, typeNamespacedName, domain)
			if err != nil && errors.IsNotFound(err) {
				resource := &operatorv1alpha1.Domain{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: operatorv1alpha1.DomainSpec{
						Name:          "example.com",
						Comment:       "example domain",
						MaxUsers:      -1,
						MaxAliases:    -1,
						MaxQuotaBytes: -1,
						SignupEnabled: false,
						Alternatives:  []string{"foo.example.com"},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &operatorv1alpha1.Domain{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance Domain")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DomainReconciler{
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

			domain := &operatorv1alpha1.Domain{}
			err = k8sClient.Get(ctx, typeNamespacedName, domain)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Status.Conditions).ToNot(BeEmpty())
		})

		It("should successfully remove the resource", func() {
			By("Reconciling the deleted resource")
			err := k8sClient.Delete(ctx, domain)
			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &DomainReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   mailuMock,
				ApiToken: "asdf",
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			domain := &operatorv1alpha1.Domain{}
			err = k8sClient.Get(ctx, typeNamespacedName, domain)
			Expect(err).To(HaveOccurred())
		})

		It("should fail without API credentials", func() {
			By("Reconciling the resource")
			mux := chi.NewMux()
			mux.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code": 401, "message":"Authorization required"}`))
			})
			httpSrv := httptest.NewServer(mux)
			DeferCleanup(httpSrv.Close)

			controllerReconciler := &DomainReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   httpSrv.URL,
				ApiToken: "asdf",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should fail with invalid API credentials", func() {
			By("Reconciling the resource")
			mux := chi.NewMux()
			mux.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code": 403, "message":"You are not authorized to access this resource"}`))
			})
			httpSrv := httptest.NewServer(mux)
			DeferCleanup(httpSrv.Close)

			controllerReconciler := &DomainReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   httpSrv.URL,
				ApiToken: "asdf",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should retry when API is unavailable", func() {
			By("Reconciling the resource")
			mux := chi.NewMux()
			mux.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"code": 503, "message":"Temporarily unavailable"}`))
			})
			httpSrv := httptest.NewServer(mux)
			DeferCleanup(httpSrv.Close)

			controllerReconciler := &DomainReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				ApiURL:   httpSrv.URL,
				ApiToken: "asdf",
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
