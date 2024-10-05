package controller_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	. "github.com/sickhub/mailu-operator/internal/controller"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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

	reconcile := func(deleted bool) (ctrl.Result, error) {
		Expect(res).NotTo(BeNil())
		Expect(controllerReconciler).NotTo(BeNil())

		typeNamespacedName := types.NamespacedName{
			Name:      res.GetName(),
			Namespace: res.GetNamespace(),
		}
		var resultErr error
		result, resultErr = controllerReconciler.Reconcile(ctx, res)

		resAfterReconciliation = &operatorv1alpha1.Alias{}
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
				res = CreateResource(operatorv1alpha1.Alias{}, name, domain).(*operatorv1alpha1.Alias)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the status, if creation fails", func() {
				prepareFindAlias(res, http.StatusNotFound)
				prepareCreateAlias(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeFalse())
				Expect(result.Requeue).To(BeTrue())
			})

			It("creates the alias, updates status and adds a finalizer", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindAlias(res, http.StatusNotFound)
				prepareCreateAlias(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindAlias(res, http.StatusServiceUnavailable)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(result.Requeue).To(BeTrue())
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})

			It("updates status, if a permanent error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindAlias(res, http.StatusBadRequest)

				_, err := reconcile(false)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Requeue).To(BeFalse())
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeFalse())
				condition := meta.FindStatusCondition(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)
				Expect(condition.Reason).To(Equal("Error"))
			})
		})

		When("updating an Alias", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				res.Spec.Comment = mockComment
				err := k8sClient.Update(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the alias", func() {
				prepareFindAlias(resAfterReconciliation, http.StatusOK)
				preparePatchAlias(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})

			It("does nothing, if there is no change", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindAlias(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				res.Spec.Comment = mockComment + "1"
				err := k8sClient.Update(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())

				prepareFindAlias(resAfterReconciliation, http.StatusOK)
				preparePatchAlias(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment + "1"))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeFalse())
				Expect(result.Requeue).To(BeTrue())
			})
		})

		When("deleting an Alias", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				err := k8sClient.Delete(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("requeues the request, if a retryable error occurs", func() {
				prepareFindAlias(res, http.StatusOK)
				prepareDeleteAlias(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeFalse())
				Expect(result.Requeue).To(BeTrue())
			})

			It("deletes the alias", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindAlias(res, http.StatusOK)
				prepareDeleteAlias(res, http.StatusOK)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.Alias{}))
			})
		})

		// Info: this must be after deleting the resource, because we're creating it again.
		When("creating an Alias that already exists", func() {
			BeforeAll(func() {
				res = CreateResource(operatorv1alpha1.Alias{}, name, domain).(*operatorv1alpha1.Alias)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds an existing alias, updates status and adds a finalizer", func() {
				Expect(res.GetFinalizers()).To(HaveLen(0))
				Expect(res.Status.Conditions).To(HaveLen(0))

				prepareFindAlias(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, AliasConditionTypeReady)).To(BeTrue())
			})
		})

		When("deleting an Alias that does not exist", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				err := k8sClient.Delete(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not delete the alias", func() {
				prepareFindAlias(res, http.StatusNotFound)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.Alias{}))
			})
		})
	})

})
