package controller_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	. "github.com/sickhub/mailu-operator/internal/controller"
)

var _ = Describe("Alias Controller", func() {
	var (
		controllerReconciler   *AliasReconciler
		res                    *operatorv1alpha1.Alias
		result                 ctrl.Result
		resAfterReconciliation *operatorv1alpha1.Alias
		name                   string
		domain                 string
	)
	ctx := context.Background()

	createResource := func(tp interface{}, name, domain string) client.Object {
		switch tp.(type) {
		case operatorv1alpha1.Alias:
			return &operatorv1alpha1.Alias{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec:       operatorv1alpha1.AliasSpec{Name: name, Domain: domain},
			}
		case operatorv1alpha1.User:
			return nil
		case operatorv1alpha1.Domain:
			return nil
		default:
			return nil
		}
	}

	reconcile := func(deleted bool) (ctrl.Result, error) {
		Expect(res).NotTo(BeNil())
		Expect(controllerReconciler).NotTo(BeNil())

		typeNamespacedName := types.NamespacedName{
			Name:      res.GetName(),
			Namespace: res.GetNamespace(),
		}
		var resultErr error
		result, resultErr = controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: typeNamespacedName,
		})

		resAfterReconciliation = &operatorv1alpha1.Alias{}
		err := k8sClient.Get(ctx, typeNamespacedName, resAfterReconciliation)
		if !deleted {
			Expect(err).ToNot(HaveOccurred())
		}

		return result, resultErr
	}

	BeforeEach(func() {
		name = "foo"
		domain = "example.com"
		mock = ghttp.NewServer()

		Expect(k8sClient).NotTo(BeNil())
		controllerReconciler = &AliasReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			ApiURL: mock.URL(),
			//ApiToken: "asdf",
		}
	})

	Context("On an empty cluster", Ordered, func() {

		When("creating an Alias", func() {
			BeforeAll(func() {
				res = createResource(operatorv1alpha1.Alias{}, name, domain).(*operatorv1alpha1.Alias)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates the alias, updates status and adds a finalizer", func() {
				prepareFindAlias(res, http.StatusNotFound)
				prepareCreateAlias(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				prepareFindAlias(res, http.StatusServiceUnavailable)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(result.Requeue).To(BeTrue())
				//Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeFalse())
			})

			It("returns an error, if a permanent error occurs", func() {
				prepareFindAlias(res, http.StatusBadRequest)

				_, err := reconcile(false)
				Expect(err).To(HaveOccurred())
			})
		})

		When("updating an Alias", func() {
			BeforeAll(func() {
				resAfterReconciliation.Spec.Comment = "some comment"
				err := k8sClient.Update(ctx, resAfterReconciliation)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the alias", func() {
				prepareFindAlias(resAfterReconciliation, http.StatusOK)
				preparePatchAlias(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal("some comment"))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})
		})

		When("receiving an error from the API", func() {
			It("updates the status upon failure", func() {
				prepareFindAlias(resAfterReconciliation, http.StatusNotFound)
				prepareCreateAlias(resAfterReconciliation, http.StatusConflict)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeFalse())
			})
		})

		When("deleting an Alias", func() {
			BeforeAll(func() {
				err := k8sClient.Delete(ctx, resAfterReconciliation)
				Expect(err).ToNot(HaveOccurred())
			})

			It("deletes the alias", func() {
				prepareFindAlias(resAfterReconciliation, http.StatusOK)
				prepareDeleteAlias(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.Alias{}))
			})
		})

	})
})
