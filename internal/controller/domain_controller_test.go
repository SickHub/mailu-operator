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
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gitlab.rootcrew.net/rootcrew/services/mailu-operator/pkg/mailu"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "gitlab.rootcrew.net/rootcrew/services/mailu-operator/api/v1alpha1"
)

var _ = Describe("Domain Controller", func() {
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
						Name:          "nonexistent.com",
						Comment:       "example domain",
						MaxUsers:      -1,
						MaxAliases:    -1,
						MaxQuotaBytes: -1,
						SignupEnabled: false,
						Alternatives:  []string{},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &operatorv1alpha1.Domain{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Domain")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
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

			domain := &operatorv1alpha1.Domain{}
			err = k8sClient.Get(ctx, typeNamespacedName, domain)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain.Status.Conditions).ToNot(BeEmpty())
		})
	})

	Context("When reconciling a deleted resource", func() {
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
						Alternatives:  []string{},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				// reconcile to create the resource
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

				k8sClient.Get(ctx, typeNamespacedName, domain) //nolint:errcheck
				Expect(domain.Status.Conditions).ToNot(BeEmpty())
				Expect(domain.ObjectMeta.Finalizers).ToNot(BeEmpty())

				// delete the resource
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				k8sClient.Get(ctx, typeNamespacedName, domain) //nolint:errcheck
				Expect(domain.ObjectMeta.DeletionTimestamp).ToNot(BeNil())
			}
		})

		AfterEach(func() {
			// resource should not be found
			resource := &operatorv1alpha1.Domain{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the deleted resource")
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

			domain := &operatorv1alpha1.Domain{}
			err = k8sClient.Get(ctx, typeNamespacedName, domain)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})

func mailuMock() string {
	mux := chi.NewMux()
	// get domains
	mux.Get("/domain", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)
		response := []mailu.DomainDetails{
			{
				Name: "example.com",
			},
			{
				Name: "foo.example.com",
			},
		}

		body, err := jsoniter.Marshal(response)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
			return
		}

		_, err = w.Write(body)
		Expect(err).NotTo(HaveOccurred())
	})

	// get specific domain
	mux.Get("/domain/example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		domain := mailu.DomainDetails{
			Name: "example.com",
		}
		body, err := jsoniter.Marshal(domain)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Get("/domain/nonexistent.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "Not found"}`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, err = w.Write(body)
		Expect(err).To(HaveOccurred())
	})

	// create domain
	mux.Post("/domain", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("post request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	// update domain
	mux.Patch("/domain/example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("patch request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	// delete domain
	mux.Delete("/domain/example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("delete request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	httpSrv := httptest.NewServer(mux)
	//t.Cleanup(func() { httpSrv.Close() })

	return httpSrv.URL
}
