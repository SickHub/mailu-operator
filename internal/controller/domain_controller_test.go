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
		result, resultErr = controllerReconciler.Reconcile(ctx, res)

		resAfterReconciliation = &operatorv1alpha1.Domain{}
		err := k8sClient.Get(ctx, typeNamespacedName, resAfterReconciliation)
		if !deleted {
			Expect(err).ToNot(HaveOccurred())
		}

		return result, resultErr
	}

	BeforeEach(func() {
		name = mockName
		domain = mockDomain
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

			It("updates the status, if creation fails", func() {
				prepareFindDomain(res, http.StatusNotFound)
				prepareCreateDomain(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeFalse())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			})

			It("creates the domain, updates status and adds a finalizer", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindDomain(res, http.StatusNotFound)
				prepareCreateDomain(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindDomain(res, http.StatusServiceUnavailable)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})

			It("updates status, if a permanent error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindDomain(res, http.StatusBadRequest)

				_, err := reconcile(false)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.RequeueAfter).To(BeNumerically("==", 0))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeFalse())
				condition := meta.FindStatusCondition(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)
				Expect(condition.Reason).To(Equal("Error"))
			})

			It("updates the status, if creation fails with conflict", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindDomain(res, http.StatusNotFound)
				prepareCreateDomain(res, http.StatusConflict)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeFalse())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			})
		})

		When("updating a Domain", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				res.Spec.Comment = mockComment
				err := k8sClient.Update(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the domain", func() {
				prepareFindDomain(resAfterReconciliation, http.StatusOK)
				preparePatchDomain(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})

			It("does nothing, if there is no change", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindDomain(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				res.Spec.Comment = mockComment + "1"
				err := k8sClient.Update(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())

				prepareFindDomain(resAfterReconciliation, http.StatusOK)
				preparePatchDomain(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment + "1"))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeFalse())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			})
		})

		When("deleting a Domain", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				err := k8sClient.Delete(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("requeues the request, if a retryable error occurs", func() {
				prepareFindDomain(res, http.StatusOK)
				prepareDeleteDomain(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeFalse())
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			})

			It("deletes the domain", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindDomain(res, http.StatusOK)
				prepareDeleteDomain(res, http.StatusOK)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.Domain{}))
			})
		})

		// Info: this must be after deleting the resource, because we're creating it again.
		When("creating a Domain that already exists", func() {
			BeforeAll(func() {
				res = CreateResource(operatorv1alpha1.Domain{}, name, domain).(*operatorv1alpha1.Domain)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds an existing domain, updates status and adds a finalizer", func() {
				Expect(res.GetFinalizers()).To(HaveLen(0))
				Expect(res.Status.Conditions).To(HaveLen(0))

				prepareFindDomain(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, DomainConditionTypeReady)).To(BeTrue())
			})
		})

		When("deleting a Domain that does not exist", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				err := k8sClient.Delete(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not delete the alias", func() {
				prepareFindDomain(res, http.StatusNotFound)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.Domain{}))
			})
		})
	})
})
