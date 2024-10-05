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
	"slices"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
//func (r *AliasReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:gocyclo
//	logr := log.FromContext(ctx)
//
//	alias := &operatorv1alpha1.Alias{}
//	err := r.Get(ctx, req.NamespacedName, alias)
//	if err != nil {
//		return ctrl.Result{}, client.IgnoreNotFound(err)
//	}
//
//	api, err := mailu.NewClient(r.ApiURL, mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
//		req.Header.Add("Authorization", "Bearer "+r.ApiToken)
//		return nil
//	}), mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
//		body := make([]byte, 0)
//		if req.Body != nil {
//			body, _ = io.ReadAll(req.Body) //nolint:errcheck
//			req.Body = io.NopCloser(bytes.NewBuffer(body))
//		}
//		logr.Info(fmt.Sprintf("request %s %s: %s", req.Method, req.URL.Path, body))
//		return nil
//	}))
//	if err != nil {
//		return ctrl.Result{}, err
//	}
//
//	email := alias.Spec.Name + "@" + alias.Spec.Domain
//
//	find, err := api.FindAlias(ctx, email)
//	if err != nil {
//		return ctrl.Result{Requeue: true}, err
//	}
//	defer find.Body.Close() //nolint:errcheck
//
//	// handle generic errors
//	switch find.StatusCode {
//	case http.StatusForbidden:
//		return ctrl.Result{}, errors.New("invalid authorization")
//
//	case http.StatusUnauthorized:
//		return ctrl.Result{}, errors.New("missing authorization")
//
//	case http.StatusBadRequest:
//		return ctrl.Result{}, errors.New("bad request")
//
//	case http.StatusServiceUnavailable:
//		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
//	}
//
//	if alias.ObjectMeta.DeletionTimestamp.IsZero() {
//		// Add a finalizer if not present
//		if !controllerutil.ContainsFinalizer(alias, FinalizerName) {
//			alias.ObjectMeta.Finalizers = append(alias.ObjectMeta.Finalizers, FinalizerName)
//			if err := r.Update(ctx, alias); err != nil {
//				//log.Error(err, "unable to update Tenant")
//				return ctrl.Result{Requeue: true}, err
//			}
//		}
//
//		// not deleted -> ensure the domain exists
//		var response *http.Response
//		switch find.StatusCode {
//		case http.StatusNotFound:
//			response, err = api.CreateAlias(ctx, mailu.Alias{
//				Email:       email,
//				Comment:     &alias.Spec.Comment,
//				Destination: &alias.Spec.Destination,
//				Wildcard:    &alias.Spec.Wildcard,
//			})
//			if err != nil {
//				return ctrl.Result{}, err
//			}
//			defer response.Body.Close() //nolint:errcheck
//
//			body, err := io.ReadAll(response.Body)
//			if err != nil {
//				return ctrl.Result{}, err
//			}
//
//			switch response.StatusCode {
//			case http.StatusBadRequest:
//				fallthrough
//			case http.StatusConflict:
//				fallthrough
//			case http.StatusNotFound:
//				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Created", "Alias create failed: "+string(body)))
//				err = r.Status().Update(ctx, alias)
//				if err != nil {
//					return ctrl.Result{Requeue: true}, err
//				}
//				logr.Error(err, fmt.Sprintf("failed to create alias: %d %s", response.StatusCode, body))
//				return ctrl.Result{Requeue: false}, nil
//			case http.StatusOK:
//				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionTrue, "Created", "Alias create successful"))
//				err = r.Status().Update(ctx, alias)
//				if err != nil {
//					return ctrl.Result{Requeue: true}, err
//				}
//				logr.Info(fmt.Sprintf("alias %s created", email))
//				return ctrl.Result{}, nil
//			default:
//				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Created", "Alias create failed: "+string(body)))
//				err = r.Status().Update(ctx, alias)
//				if err != nil {
//					return ctrl.Result{Requeue: true}, err
//				}
//				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to create alias: %d %s", response.StatusCode, body)
//			}
//
//		case http.StatusOK:
//			body, err := io.ReadAll(find.Body)
//			if err != nil {
//				return ctrl.Result{Requeue: true}, err
//			}
//
//			foundAlias := mailu.Alias{}
//			err = json.Unmarshal(body, &foundAlias)
//			if err != nil {
//				return ctrl.Result{Requeue: true}, err
//			}
//
//			ali := mailu.Alias{
//				Email:       email,
//				Comment:     &alias.Spec.Comment,
//				Destination: &alias.Spec.Destination,
//				Wildcard:    &alias.Spec.Wildcard,
//			}
//
//			// no change
//			if reflect.DeepEqual(foundAlias, ali) {
//				return ctrl.Result{}, nil
//			}
//
//			response, err = api.UpdateAlias(ctx, email, ali)
//			if err != nil {
//				return ctrl.Result{}, err
//			}
//			defer response.Body.Close() //nolint:errcheck
//
//			body, err = io.ReadAll(response.Body)
//			if err != nil {
//				return ctrl.Result{Requeue: true}, err
//			}
//
//			switch response.StatusCode {
//			case http.StatusBadRequest:
//				fallthrough
//			case http.StatusConflict:
//				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Updated", "Alias update failed: "+string(body)))
//				err = r.Status().Update(ctx, alias)
//				if err != nil {
//					return ctrl.Result{Requeue: true}, err
//				}
//				logr.Error(err, fmt.Sprintf("failed to create alias: %d %s", response.StatusCode, body))
//				return ctrl.Result{Requeue: false}, nil
//			case http.StatusOK:
//				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionTrue, "Updated", "Alias update successful"))
//				err = r.Status().Update(ctx, alias)
//				if err != nil {
//					return ctrl.Result{Requeue: true}, err
//				}
//				logr.Info(fmt.Sprintf("alias %s updated", email))
//				return ctrl.Result{}, nil
//			default:
//				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Updated", "Alias update failed: "+string(body)))
//				err = r.Status().Update(ctx, alias)
//				if err != nil {
//					return ctrl.Result{Requeue: true}, err
//				}
//				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to update alias: %d %s", response.StatusCode, body)
//			}
//
//		default:
//			err = errors.New("unexpected status code")
//			logr.Error(err, fmt.Sprintf("unexpected status code: %d", find.StatusCode))
//			return ctrl.Result{}, err
//		}
//	}
//
//	if controllerutil.ContainsFinalizer(alias, FinalizerName) {
//		// Domain removed -> delete
//		if find.StatusCode != http.StatusNotFound {
//			res, err := api.DeleteAlias(ctx, email)
//			if err != nil {
//				return ctrl.Result{Requeue: true}, err
//			}
//			if res.StatusCode != http.StatusOK {
//				return ctrl.Result{Requeue: true}, nil
//			}
//		}
//
//		controllerutil.RemoveFinalizer(alias, FinalizerName)
//		if err := r.Update(ctx, alias); err != nil {
//			return ctrl.Result{}, err
//		}
//		logr.Info(fmt.Sprintf("alias %s deleted", email))
//	}
//
//	return ctrl.Result{}, nil
//}

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

	if alias.DeletionTimestamp == nil && !slices.Contains(alias.Finalizers, FinalizerName) {
		alias.Finalizers = append(alias.Finalizers, FinalizerName)
	}

	result, err := r.reconcile(ctx, alias)
	if err != nil {
		return result, err
	}

	if aliasOriginal.DeletionTimestamp != nil {
		// remove finalizer if deleted and reconciliation was successful
		alias.Finalizers = []string{}
		for _, f := range aliasOriginal.Finalizers {
			if f == FinalizerName {
				continue
			}
			alias.Finalizers = append(alias.Finalizers, f)
		}
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
