package controller

import (
	"bytes"
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
	AliasConditionTypeReady = "AliasReady"
)

// AliasReconciler reconciles a Alias object
type AliasReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ApiURL    string
	ApiToken  string
	ApiClient *mailu.Client
}

//+kubebuilder:rbac:groups=operator.mailu.io,resources=aliases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.mailu.io,resources=aliases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.mailu.io,resources=aliases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *AliasReconciler) Reconcile(ctx context.Context, alias *operatorv1alpha1.Alias) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	aliasOriginal := alias.DeepCopy()

	// apply patches at the end, before returning
	defer func() {
		if err := r.Client.Patch(ctx, alias.DeepCopy(), client.MergeFrom(aliasOriginal)); err != nil {
			logr.Error(err, "failed to patch resource")
		}
		if err := r.Client.Status().Patch(ctx, alias.DeepCopy(), client.MergeFrom(aliasOriginal)); err != nil {
			logr.Error(err, "failed to patch resource status")
		}
	}()

	if alias.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(alias, FinalizerName) {
		controllerutil.AddFinalizer(alias, FinalizerName)
	}

	result, err := r.reconcile(ctx, alias)
	if err != nil {
		return result, err
	}

	if aliasOriginal.DeletionTimestamp != nil && result.Requeue == false {
		controllerutil.RemoveFinalizer(alias, FinalizerName)
	}

	return result, nil
}

func (r *AliasReconciler) reconcile(ctx context.Context, alias *operatorv1alpha1.Alias) (ctrl.Result, error) {
	logr := log.FromContext(ctx)

	if r.ApiClient == nil {
		api, err := mailu.NewClient(r.ApiURL, mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+r.ApiToken)
			return nil
		}), mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			// TODO: make this optional
			body := make([]byte, 0)
			if req.Body != nil {
				body, _ = io.ReadAll(req.Body) //nolint:errcheck
				req.Body = io.NopCloser(bytes.NewBuffer(body))
			}
			logr.Info(fmt.Sprintf("request %s %s: %s", req.Method, req.URL.Path, body))
			return nil
		}))
		if err != nil {
			return ctrl.Result{}, err
		}
		r.ApiClient = api
	}

	foundAlias, retry, err := r.getAlias(ctx, alias)
	if err != nil {
		if retry {
			logr.Info(fmt.Errorf("failed to get alias, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to get alias")
		meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		return ctrl.Result{}, nil
	}

	if alias.DeletionTimestamp != nil {
		if foundAlias == nil {
			// no need to delete it, if it does not exist
			return ctrl.Result{}, nil
		}
		return r.delete(ctx, alias)
	}

	if foundAlias == nil {
		return r.create(ctx, alias)
	}

	return r.update(ctx, alias, foundAlias)
}

func (r *AliasReconciler) create(ctx context.Context, alias *operatorv1alpha1.Alias) (ctrl.Result, error) {
	logr := log.FromContext(ctx)
	logr.Info("create alias")

	retry, err := r.createAlias(ctx, alias)
	if err != nil {
		if retry {
			logr.Info(fmt.Errorf("failed to create alias, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to create alias")
		meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionTrue, "Created", "Alias created in MailU"))

	return ctrl.Result{}, nil
}

func (r *AliasReconciler) update(ctx context.Context, alias *operatorv1alpha1.Alias, apiAlias *mailu.Alias) (ctrl.Result, error) {
	logr := log.FromContext(ctx)
	logr.Info("update alias")

	newAlias := mailu.Alias{
		Email:       alias.Spec.Name + "@" + alias.Spec.Domain,
		Comment:     &alias.Spec.Comment,
		Destination: &alias.Spec.Destination,
		Wildcard:    &alias.Spec.Wildcard,
	}

	if reflect.DeepEqual(newAlias, apiAlias) {
		return ctrl.Result{}, nil
	}

	retry, err := r.updateAlias(ctx, alias)
	if err != nil {
		if retry {
			logr.Info(fmt.Errorf("failed to update alias, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to update alias")
		meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionTrue, "Updated", "Alias updated in MailU"))

	return ctrl.Result{}, nil
}

func (r *AliasReconciler) delete(ctx context.Context, alias *operatorv1alpha1.Alias) (ctrl.Result, error) {
	logr := log.FromContext(ctx)
	logr.Info("delete alias")

	retry, err := r.deleteAlias(ctx, alias)
	if err != nil {
		if retry {
			logr.Info(fmt.Errorf("failed to delete alias, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to delete alias")
		meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AliasReconciler) getAlias(ctx context.Context, alias *operatorv1alpha1.Alias) (*mailu.Alias, bool, error) {
	found, err := r.ApiClient.FindAlias(ctx, alias.Spec.Name+"@"+alias.Spec.Domain)
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
		foundAlias := &mailu.Alias{}
		err = json.Unmarshal(body, &foundAlias)
		if err != nil {
			return nil, true, err
		}

		return foundAlias, false, nil
	case http.StatusNotFound:
		return nil, false, nil
	case http.StatusBadRequest:
		return nil, false, errors.New("bad request")
	case http.StatusServiceUnavailable:
		return nil, true, errors.New("service unavailable")
	}
	return nil, false, errors.New("unknown status: " + strconv.Itoa(found.StatusCode))
}

func (r *AliasReconciler) createAlias(ctx context.Context, alias *operatorv1alpha1.Alias) (bool, error) {
	res, err := r.ApiClient.CreateAlias(ctx, mailu.Alias{
		Email:       alias.Spec.Name + "@" + alias.Spec.Domain,
		Comment:     &alias.Spec.Comment,
		Destination: &alias.Spec.Destination,
		Wildcard:    &alias.Spec.Wildcard,
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
	case http.StatusOK:
		return false, nil
	case http.StatusConflict:
		return false, errors.New("alias already exists")
	case http.StatusInternalServerError:
		return false, errors.New("internal server error")
	case http.StatusServiceUnavailable:
		return true, nil
	}

	return false, errors.New("unknown status: " + strconv.Itoa(res.StatusCode))
}

func (r *AliasReconciler) updateAlias(ctx context.Context, alias *operatorv1alpha1.Alias) (bool, error) {
	email := alias.Spec.Name + "@" + alias.Spec.Domain
	res, err := r.ApiClient.UpdateAlias(ctx, email, mailu.Alias{
		Email:       email,
		Comment:     &alias.Spec.Comment,
		Destination: &alias.Spec.Destination,
		Wildcard:    &alias.Spec.Wildcard,
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
	case http.StatusOK:
		return false, nil
	case http.StatusInternalServerError:
		return false, errors.New("internal server error")
	case http.StatusServiceUnavailable:
		return true, nil
	}

	return false, errors.New("unknown status: " + strconv.Itoa(res.StatusCode))
}

func (r *AliasReconciler) deleteAlias(ctx context.Context, alias *operatorv1alpha1.Alias) (bool, error) {
	res, err := r.ApiClient.DeleteAlias(ctx, alias.Spec.Name+"@"+alias.Spec.Domain)
	if err != nil {
		return false, err
	}
	defer res.Body.Close() //nolint:errcheck

	_, err = io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	switch res.StatusCode {
	case http.StatusOK:
		return false, nil
	case http.StatusInternalServerError:
		return false, errors.New("internal server error")
	case http.StatusServiceUnavailable:
		return true, nil
	}

	return false, errors.New("unknown status: " + strconv.Itoa(res.StatusCode))
}

func getReadyCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    AliasConditionTypeReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AliasReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Alias{}).
		Complete(reconcile.AsReconciler(r.Client, r))
}
