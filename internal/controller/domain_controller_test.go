package controller_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	. "github.com/sickhub/mailu-operator/internal/controller"
)

var _ = Describe("Domain Controller", func() {
	var (
		controllerReconciler   *DomainReconciler
		res                    *operatorv1alpha1.Domain
		result                 ctrl.Result
		resAfterReconciliation *operatorv1alpha1.Domain
		name                   string
		domain                 string
	)
	ctx := context.Background()

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

		resAfterReconciliation = &operatorv1alpha1.Domain{}
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
		controllerReconciler = &DomainReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			ApiURL: mock.URL(),
			//ApiToken: "asdf",
		}
	})

	Context("On an empty cluster", Ordered, func() {

		When("creating a Domain", func() {
			BeforeAll(func() {
				res = CreateResource(operatorv1alpha1.Domain{}, name, domain).(*operatorv1alpha1.Domain)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates the domain, updates status and adds a finalizer", func() {
				prepareFindDomain(res, http.StatusNotFound)
				prepareCreateDomain(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})
		})

		When("updating a Domain", func() {
			BeforeAll(func() {
				resAfterReconciliation.Spec.Comment = "some comment"
				err := k8sClient.Update(ctx, resAfterReconciliation)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the domain", func() {
				prepareFindDomain(resAfterReconciliation, http.StatusOK)
				preparePatchDomain(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal("some comment"))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})
		})

		When("receiving an error from the API", func() {
			It("updates the status upon failure", func() {
				prepareFindDomain(resAfterReconciliation, http.StatusNotFound)
				prepareCreateDomain(resAfterReconciliation, http.StatusConflict)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeFalse())
			})
		})

		When("deleting an Domain", func() {
			BeforeAll(func() {
				err := k8sClient.Delete(ctx, resAfterReconciliation)
				Expect(err).ToNot(HaveOccurred())
			})

			It("deletes the domain", func() {
				prepareFindDomain(resAfterReconciliation, http.StatusOK)
				prepareDeleteDomain(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.Domain{}))
			})
		})
	})
})
