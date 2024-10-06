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

var _ = Describe("User Controller", func() {
	var (
		controllerReconciler   *UserReconciler
		res                    *operatorv1alpha1.User
		result                 ctrl.Result
		resAfterReconciliation *operatorv1alpha1.User
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

		resAfterReconciliation = &operatorv1alpha1.User{}
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
		controllerReconciler = &UserReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			ApiURL: mock.URL(),
			//ApiToken: "asdf",
		}
	})

	Context("On an empty cluster", Ordered, func() {

		When("creating a User", func() {
			BeforeAll(func() {
				res = CreateResource(operatorv1alpha1.User{}, name, domain).(*operatorv1alpha1.User)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the status, if creation fails", func() {
				prepareFindUser(res, http.StatusNotFound)
				prepareCreateUser(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeFalse())
				Expect(result.Requeue).To(BeTrue())
			})

			It("creates the user, updates status and adds a finalizer", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindUser(res, http.StatusNotFound)
				prepareCreateUser(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindUser(res, http.StatusServiceUnavailable)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(result.Requeue).To(BeTrue())
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})

			It("updates status, if a permanent error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindUser(res, http.StatusBadRequest)

				_, err := reconcile(false)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Requeue).To(BeFalse())
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeFalse())
				condition := meta.FindStatusCondition(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)
				Expect(condition.Reason).To(Equal("Error"))
			})

			It("updates the status, if creation fails with conflict", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindUser(res, http.StatusNotFound)
				prepareCreateUser(res, http.StatusConflict)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
				Expect(result.Requeue).To(BeTrue())
			})
		})

		When("updating a User", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				res.Spec.Comment = mockComment
				err := k8sClient.Update(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("updates the user", func() {
				prepareFindUser(resAfterReconciliation, http.StatusOK)
				preparePatchUser(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})

			It("does nothing, if there is no change", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindUser(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})

			It("requeues the request, if a retryable error occurs", func() {
				res = resAfterReconciliation.DeepCopy()
				res.Spec.Comment = mockComment + "1"
				err := k8sClient.Update(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())

				prepareFindUser(resAfterReconciliation, http.StatusOK)
				preparePatchUser(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.Spec.Comment).To(Equal(mockComment + "1"))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeFalse())
				Expect(result.Requeue).To(BeTrue())
			})
		})

		When("deleting a User", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				err := k8sClient.Delete(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("requeues the request, if a retryable error occurs", func() {
				prepareFindUser(res, http.StatusOK)
				prepareDeleteUser(res, http.StatusServiceUnavailable)

				result, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeFalse())
				Expect(result.Requeue).To(BeTrue())
			})

			It("deletes the user", func() {
				res = resAfterReconciliation.DeepCopy()
				prepareFindUser(resAfterReconciliation, http.StatusOK)
				prepareDeleteUser(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.User{}))
			})
		})

		// Info: this must be after deleting the resource, because we're creating it again.
		When("creating a User that already exists", func() {
			BeforeAll(func() {
				res = CreateResource(operatorv1alpha1.User{}, name, domain).(*operatorv1alpha1.User)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds an existing user, updates status and adds a finalizer", func() {
				Expect(res.GetFinalizers()).To(HaveLen(0))
				Expect(res.Status.Conditions).To(HaveLen(0))

				prepareFindUser(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})
		})

		When("deleting a User that does not exist", func() {
			BeforeAll(func() {
				res = resAfterReconciliation.DeepCopy()
				err := k8sClient.Delete(ctx, res)
				Expect(err).ToNot(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: res.GetName(), Namespace: res.GetNamespace()}, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not delete the alias", func() {
				prepareFindUser(res, http.StatusNotFound)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.User{}))
			})
		})
	})
})
