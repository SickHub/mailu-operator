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
	"time"

	"github.com/sickhub/mailu-operator/pkg/mailu"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
)

const (
	AliasConditionTypeReady = "AliasReady"
)

// AliasReconciler reconciles a Alias object
type AliasReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ApiURL   string
	ApiToken string
}

//+kubebuilder:rbac:groups=operator.mailu.io,resources=aliases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.mailu.io,resources=aliases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.mailu.io,resources=aliases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *AliasReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:gocyclo
	logr := log.FromContext(ctx)

	alias := &operatorv1alpha1.Alias{}
	err := r.Get(ctx, req.NamespacedName, alias)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	api, err := mailu.NewClient(r.ApiURL, mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+r.ApiToken)
		return nil
	}), mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
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

	email := alias.Spec.Name + "@" + alias.Spec.Domain

	find, err := api.FindAlias(ctx, email)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	defer find.Body.Close() //nolint:errcheck

	// handle generic errors
	switch find.StatusCode {
	case http.StatusForbidden:
		return ctrl.Result{}, errors.New("invalid authorization")

	case http.StatusUnauthorized:
		return ctrl.Result{}, errors.New("missing authorization")

	case http.StatusBadRequest:
		return ctrl.Result{}, errors.New("bad request")

	case http.StatusServiceUnavailable:
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
	}

	if alias.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add a finalizer if not present
		if !controllerutil.ContainsFinalizer(alias, FinalizerName) {
			alias.ObjectMeta.Finalizers = append(alias.ObjectMeta.Finalizers, FinalizerName)
			if err := r.Update(ctx, alias); err != nil {
				//log.Error(err, "unable to update Tenant")
				return ctrl.Result{Requeue: true}, err
			}
		}

		// not deleted -> ensure the domain exists
		var response *http.Response
		switch find.StatusCode {
		case http.StatusNotFound:
			response, err = api.CreateAlias(ctx, mailu.Alias{
				Email:       email,
				Comment:     &alias.Spec.Comment,
				Destination: &alias.Spec.Destination,
				Wildcard:    &alias.Spec.Wildcard,
			})
			if err != nil {
				return ctrl.Result{}, err
			}
			defer response.Body.Close() //nolint:errcheck

			body, err := io.ReadAll(response.Body)
			if err != nil {
				return ctrl.Result{}, err
			}

			switch response.StatusCode {
			case http.StatusBadRequest:
				fallthrough
			case http.StatusConflict:
				fallthrough
			case http.StatusNotFound:
				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Created", "Alias create failed: "+string(body)))
				err = r.Status().Update(ctx, alias)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Error(err, fmt.Sprintf("failed to create alias: %d %s", response.StatusCode, body))
				return ctrl.Result{Requeue: false}, nil
			case http.StatusOK:
				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionTrue, "Created", "Alias create successful"))
				err = r.Status().Update(ctx, alias)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Info(fmt.Sprintf("alias %s created", email))
				return ctrl.Result{}, nil
			default:
				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Created", "Alias create failed: "+string(body)))
				err = r.Status().Update(ctx, alias)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to create alias: %d %s", response.StatusCode, body)
			}

		case http.StatusOK:
			body, err := io.ReadAll(find.Body)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			foundAlias := mailu.Alias{}
			err = json.Unmarshal(body, &foundAlias)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			ali := mailu.Alias{
				Email:       email,
				Comment:     &alias.Spec.Comment,
				Destination: &alias.Spec.Destination,
				Wildcard:    &alias.Spec.Wildcard,
			}

			// no change
			if reflect.DeepEqual(foundAlias, ali) {
				return ctrl.Result{}, nil
			}

			response, err = api.UpdateAlias(ctx, email, ali)
			if err != nil {
				return ctrl.Result{}, err
			}
			defer response.Body.Close() //nolint:errcheck

			body, err = io.ReadAll(response.Body)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			switch response.StatusCode {
			case http.StatusBadRequest:
				fallthrough
			case http.StatusConflict:
				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Updated", "Alias update failed: "+string(body)))
				err = r.Status().Update(ctx, alias)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Error(err, fmt.Sprintf("failed to create alias: %d %s", response.StatusCode, body))
				return ctrl.Result{Requeue: false}, nil
			case http.StatusOK:
				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionTrue, "Updated", "Alias update successful"))
				err = r.Status().Update(ctx, alias)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Info(fmt.Sprintf("alias %s updated", email))
				return ctrl.Result{}, nil
			default:
				meta.SetStatusCondition(&alias.Status.Conditions, getReadyCondition(metav1.ConditionFalse, "Updated", "Alias update failed: "+string(body)))
				err = r.Status().Update(ctx, alias)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to update alias: %d %s", response.StatusCode, body)
			}

		default:
			err = errors.New("unexpected status code")
			logr.Error(err, fmt.Sprintf("unexpected status code: %d", find.StatusCode))
			return ctrl.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(alias, FinalizerName) {
		// Domain removed -> delete
		if find.StatusCode != http.StatusNotFound {
			res, err := api.DeleteAlias(ctx, email)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			if res.StatusCode != http.StatusOK {
				return ctrl.Result{Requeue: true}, nil
			}
		}

		controllerutil.RemoveFinalizer(alias, FinalizerName)
		if err := r.Update(ctx, alias); err != nil {
			return ctrl.Result{}, err
		}
		logr.Info(fmt.Sprintf("alias %s deleted", email))
	}

	return ctrl.Result{}, nil
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
		Complete(r)
}
