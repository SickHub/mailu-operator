package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	"github.com/sickhub/mailu-operator/pkg/mailu"
)

const (
	DomainConditionTypeReady = "DomainReady"
)

// DomainReconciler reconciles a Domain object
type DomainReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ApiURL    string
	ApiToken  string
	ApiClient *mailu.Client
}

//+kubebuilder:rbac:groups=operator.mailu.io,resources=domains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.mailu.io,resources=domains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.mailu.io,resources=domains/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *DomainReconciler) Reconcile(ctx context.Context, domain *operatorv1alpha1.Domain) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	domainOriginal := domain.DeepCopy()

	// apply patches at the end, before returning
	defer func() {
		if err := r.Patch(ctx, domain.DeepCopy(), client.MergeFrom(domainOriginal)); err != nil {
			logr.Error(err, "failed to patch resource")
		}
		if err := r.Status().Patch(ctx, domain.DeepCopy(), client.MergeFrom(domainOriginal)); err != nil {
			logr.Error(err, "failed to patch resource status")
		}
	}()

	if domain.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(domain, FinalizerName) {
		controllerutil.AddFinalizer(domain, FinalizerName)
	}

	result, err := r.reconcile(ctx, domain)
	if err != nil {
		return result, err
	}

	if domainOriginal.DeletionTimestamp != nil && !result.Requeue {
		controllerutil.RemoveFinalizer(domain, FinalizerName)
	}

	return result, nil
}

func (r *DomainReconciler) reconcile(ctx context.Context, domain *operatorv1alpha1.Domain) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	if r.ApiClient == nil {
		api, err := mailu.NewClient(r.ApiURL, mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+r.ApiToken)
			return nil
		}))
		if err != nil {
			return ctrl.Result{}, err
		}
		r.ApiClient = api
	}

	foundDomain, retry, err := r.getDomain(ctx, domain)
	if err != nil {
		if retry {
			logr.Info(fmt.Errorf("failed to get domain, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		// we explicitly set the error in the status only on a permanent (non-retryable) error
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		logr.Error(err, "failed to get domain")
		return ctrl.Result{}, nil
	}

	if domain.DeletionTimestamp != nil {
		if foundDomain == nil {
			// no need to delete it, if it does not exist
			return ctrl.Result{}, nil
		}
		return r.delete(ctx, domain)
	}

	if foundDomain == nil {
		return r.create(ctx, domain)
	}

	return r.update(ctx, domain, foundDomain)
}

func (r *DomainReconciler) create(ctx context.Context, domain *operatorv1alpha1.Domain) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	retry, err := r.createDomain(ctx, domain)
	if err != nil {
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		if retry {
			logr.Info(fmt.Errorf("failed to create domain, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to create domain")
		return ctrl.Result{}, err
	}

	if !retry {
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionTrue, "Created", "Domain created in MailU"))
		logr.Info("created domain")
	}

	return ctrl.Result{Requeue: retry}, nil
}

func (r *DomainReconciler) update(ctx context.Context, domain *operatorv1alpha1.Domain, apiDomain *mailu.Domain) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	newDomain := mailu.Domain{
		Name:          domain.Spec.Name,
		Alternatives:  &domain.Spec.Alternatives,
		Comment:       &domain.Spec.Comment,
		MaxAliases:    &domain.Spec.MaxAliases,
		MaxQuotaBytes: &domain.Spec.MaxQuotaBytes,
		MaxUsers:      &domain.Spec.MaxUsers,
		SignupEnabled: &domain.Spec.SignupEnabled,
	}

	jsonNew, _ := json.Marshal(newDomain) //nolint:errcheck
	jsonOld, _ := json.Marshal(apiDomain) //nolint:errcheck

	if reflect.DeepEqual(jsonNew, jsonOld) {
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionTrue, "Updated", "Domain updated in MailU"))
		logr.Info("domain is up to date, no change needed")
		return ctrl.Result{}, nil
	}

	retry, err := r.updateDomain(ctx, newDomain)
	if err != nil {
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		if retry {
			logr.Info(fmt.Errorf("failed to update domain, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to update domain")
		return ctrl.Result{}, err
	}

	if !retry {
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionTrue, "Updated", "Domain updated in MailU"))
		logr.Info("updated domain")
	}

	return ctrl.Result{Requeue: retry}, nil
}

func (r *DomainReconciler) delete(ctx context.Context, domain *operatorv1alpha1.Domain) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	retry, err := r.deleteDomain(ctx, domain)
	if err != nil {
		meta.SetStatusCondition(&domain.Status.Conditions, getDomainReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		if retry {
			logr.Info(fmt.Errorf("failed to delete domain, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to delete domain")
		return ctrl.Result{}, err
	}

	if !retry {
		logr.Info("deleted domain")
	}

	return ctrl.Result{Requeue: retry}, nil
}

func (r *DomainReconciler) getDomain(ctx context.Context, domain *operatorv1alpha1.Domain) (*mailu.Domain, bool, error) {
	found, err := r.ApiClient.FindDomain(ctx, domain.Spec.Name)
	if err != nil {
		return nil, false, err
	}
	defer found.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(found.Body)
	if err != nil {
		return nil, true, err
	}

	switch found.StatusCode {
	case http.StatusOK:
		foundDomain := &mailu.Domain{}
		err = json.Unmarshal(body, &foundDomain)
		if err != nil {
			return nil, true, err
		}

		return foundDomain, false, nil
	case http.StatusNotFound:
		return nil, false, nil
	case http.StatusBadRequest:
		return nil, false, errors.New("bad request")
	case http.StatusBadGateway:
		fallthrough
	case http.StatusGatewayTimeout:
		return nil, true, errors.New("gateway timeout")
	case http.StatusServiceUnavailable:
		return nil, true, errors.New("service unavailable")
	}
	return nil, false, errors.New("unknown status: " + strconv.Itoa(found.StatusCode))
}

func (r *DomainReconciler) createDomain(ctx context.Context, domain *operatorv1alpha1.Domain) (bool, error) {
	res, err := r.ApiClient.CreateDomain(ctx, mailu.Domain{
		Name:          domain.Spec.Name,
		Comment:       &domain.Spec.Comment,
		MaxUsers:      &domain.Spec.MaxUsers,
		MaxAliases:    &domain.Spec.MaxAliases,
		MaxQuotaBytes: &domain.Spec.MaxQuotaBytes,
		SignupEnabled: &domain.Spec.SignupEnabled,
	})
	if err != nil {
		return false, err
	}
	defer res.Body.Close() //nolint:errcheck

	_, err = io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	switch res.StatusCode {
	case http.StatusCreated:
		fallthrough
	case http.StatusOK:
		return false, nil
	case http.StatusConflict:
		// treat conflict as success -> requeue will trigger an update
		return true, nil
	case http.StatusInternalServerError:
		return false, errors.New("internal server error")
	case http.StatusBadGateway:
		fallthrough
	case http.StatusGatewayTimeout:
		return true, errors.New("gateway timeout")
	case http.StatusServiceUnavailable:
		return true, errors.New("service unavailable")
	}

	return false, errors.New("unknown status: " + strconv.Itoa(res.StatusCode))
}

func (r *DomainReconciler) updateDomain(ctx context.Context, newDomain mailu.Domain) (bool, error) {
	res, err := r.ApiClient.UpdateDomain(ctx, newDomain.Name, newDomain)
	if err != nil {
		return false, err
	}
	defer res.Body.Close() //nolint:errcheck

	_, err = io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	switch res.StatusCode {
	case http.StatusNoContent:
		fallthrough
	case http.StatusOK:
		return false, nil
	case http.StatusInternalServerError:
		return false, errors.New("internal server error")
	case http.StatusBadGateway:
		fallthrough
	case http.StatusGatewayTimeout:
		return true, errors.New("gateway timeout")
	case http.StatusServiceUnavailable:
		return true, errors.New("service unavailable")
	}

	return false, errors.New("unknown status: " + strconv.Itoa(res.StatusCode))
}

func (r *DomainReconciler) deleteDomain(ctx context.Context, domain *operatorv1alpha1.Domain) (bool, error) {
	res, err := r.ApiClient.DeleteDomain(ctx, domain.Spec.Name)
	if err != nil {
		return false, err
	}
	defer res.Body.Close() //nolint:errcheck

	_, err = io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	switch res.StatusCode {
	case http.StatusNotFound:
		fallthrough
	case http.StatusOK:
		return false, nil
	case http.StatusInternalServerError:
		return false, errors.New("internal server error")
	case http.StatusBadGateway:
		fallthrough
	case http.StatusGatewayTimeout:
		return true, errors.New("gateway timeout")
	case http.StatusServiceUnavailable:
		return true, errors.New("service unavailable")
	}

	return false, errors.New("unknown status: " + strconv.Itoa(res.StatusCode))
}

func getDomainReadyCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    DomainConditionTypeReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DomainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Domain{}).
		Complete(reconcile.AsReconciler(r.Client, r))
}
