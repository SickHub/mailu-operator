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

var _ = Describe("User Controller", func() {
	var (
		controllerReconciler   *UserReconciler
		res                    *operatorv1alpha1.User
		result                 ctrl.Result
		resAfterReconciliation *operatorv1alpha1.User
		name                   string
		user                   string
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

		resAfterReconciliation = &operatorv1alpha1.User{}
		err := k8sClient.Get(ctx, typeNamespacedName, resAfterReconciliation)
		if !deleted {
			Expect(err).ToNot(HaveOccurred())
		}

		return result, resultErr
	}

	BeforeEach(func() {
		name = "foo"
		user = "example.com"
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
				res = CreateResource(operatorv1alpha1.User{}, name, user).(*operatorv1alpha1.User)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates the user, updates status and adds a finalizer", func() {
				prepareFindUser(res, http.StatusNotFound)
				prepareCreateUser(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})
		})

		When("creating a User that already exists", func() {
			BeforeAll(func() {
				res = CreateResource(operatorv1alpha1.User{}, "existing.com", "existing.com").(*operatorv1alpha1.User)
				err := k8sClient.Create(ctx, res)
				Expect(err).ToNot(HaveOccurred())
			})

			It("finds an existing user, updates status and adds a finalizer", func() {
				prepareFindUser(res, http.StatusOK)
				preparePatchUser(res, http.StatusOK)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(resAfterReconciliation.Status.Conditions).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeTrue())
			})
		})

		When("updating a User", func() {
			BeforeAll(func() {
				resAfterReconciliation.Spec.Comment = mockComment
				err := k8sClient.Update(ctx, resAfterReconciliation)
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
		})

		When("receiving an error from the API", func() {
			It("updates the status upon failure", func() {
				prepareFindUser(resAfterReconciliation, http.StatusNotFound)
				prepareCreateUser(resAfterReconciliation, http.StatusConflict)

				_, err := reconcile(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation.GetFinalizers()).To(HaveLen(1))
				Expect(meta.IsStatusConditionTrue(resAfterReconciliation.Status.Conditions, UserConditionTypeReady)).To(BeFalse())
			})
		})

		When("deleting an User", func() {
			BeforeAll(func() {
				err := k8sClient.Delete(ctx, resAfterReconciliation)
				Expect(err).ToNot(HaveOccurred())
			})

			It("deletes the user", func() {
				prepareFindUser(resAfterReconciliation, http.StatusOK)
				prepareDeleteUser(resAfterReconciliation, http.StatusOK)

				_, err := reconcile(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(resAfterReconciliation).To(BeComparableTo(&operatorv1alpha1.User{}))
			})
		})
	})
})
