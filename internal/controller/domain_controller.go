/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"gitlab.rootcrew.net/rootcrew/services/mailu-operator/pkg/mailu"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "gitlab.rootcrew.net/rootcrew/services/mailu-operator/api/v1alpha1"
)

const (
	finalizerName = "operator.mailu.io/finalizer"
)

// DomainReconciler reconciles a Domain object
type DomainReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ApiURL   string
	ApiToken string
}

//+kubebuilder:rbac:groups=operator.mailu.io,resources=domains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.mailu.io,resources=domains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.mailu.io,resources=domains/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Domain object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *DomainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:gocyclo
	logr := log.FromContext(ctx)

	domain := &operatorv1alpha1.Domain{}
	err := r.Get(ctx, req.NamespacedName, domain)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	api, err := mailu.NewClient(r.ApiURL, mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+r.ApiToken)
		return nil
	}), mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		logr.Info(fmt.Sprintf("request: %+v", req))
		return nil
	}))
	if err != nil {
		return ctrl.Result{}, err
	}

	find, err := api.FindDomain(ctx, domain.Spec.Name)
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
		return ctrl.Result{Requeue: true}, nil
	}

	if domain.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add a finalizer if not present
		if !controllerutil.ContainsFinalizer(domain, finalizerName) {
			domain.ObjectMeta.Finalizers = append(domain.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, domain); err != nil {
				//log.Error(err, "unable to update Tenant")
				return ctrl.Result{Requeue: true}, err
			}
		}

		// not deleted -> ensure the domain exists
		var response *http.Response
		switch find.StatusCode {
		case http.StatusNotFound:
			response, err = api.CreateDomain(ctx, mailu.Domain{
				Name:          domain.Spec.Name,
				Comment:       &domain.Spec.Comment,
				MaxUsers:      &domain.Spec.MaxUsers,
				MaxAliases:    &domain.Spec.MaxAliases,
				MaxQuotaBytes: &domain.Spec.MaxQuotaBytes,
				SignupEnabled: &domain.Spec.SignupEnabled,
				Alternatives:  &domain.Spec.Alternatives,
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
				return ctrl.Result{}, errors.New("bad request")
			case http.StatusConflict:
				return ctrl.Result{}, fmt.Errorf("conflicting domain / alternative during create: %d %s", response.StatusCode, body)
			case http.StatusOK:
				meta.SetStatusCondition(&domain.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionTrue, Reason: "Created", Message: "Domain created"})
				err = r.Status().Update(ctx, domain)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			default:
				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to create domain: %d %s", response.StatusCode, body)
			}

		case http.StatusOK:
			body, err := io.ReadAll(find.Body)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			foundDom := mailu.Domain{}
			err = json.Unmarshal(body, &foundDom)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			dom := mailu.Domain{
				Name:          domain.Spec.Name,
				Comment:       &domain.Spec.Comment,
				MaxUsers:      &domain.Spec.MaxUsers,
				MaxAliases:    &domain.Spec.MaxAliases,
				MaxQuotaBytes: &domain.Spec.MaxQuotaBytes,
				SignupEnabled: &domain.Spec.SignupEnabled,
				Alternatives:  &domain.Spec.Alternatives,
			}

			// no change
			if reflect.DeepEqual(foundDom, dom) {
				return ctrl.Result{}, nil
			}

			response, err = api.UpdateDomain(ctx, domain.Spec.Name, dom)
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
				return ctrl.Result{}, errors.New("bad request")
			case http.StatusConflict:
				return ctrl.Result{}, fmt.Errorf("conflicting domain / alternative during update: %d %s", response.StatusCode, body)
			case http.StatusOK:
				meta.SetStatusCondition(&domain.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionTrue, Reason: "Updated", Message: "Domain updated"})
				err = r.Status().Update(ctx, domain)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{}, nil
			default:
				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to update domain: %d %s", response.StatusCode, body)
			}

		default:
			err = errors.New("unexpected status code")
			logr.Error(err, fmt.Sprintf("unexpected status code: %d", find.StatusCode))
			return ctrl.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(domain, finalizerName) {
		// Domain removed -> delete
		if find.StatusCode != http.StatusNotFound {
			res, err := api.DeleteDomain(ctx, domain.Spec.Name)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			if res.StatusCode != http.StatusOK {
				return ctrl.Result{Requeue: true}, nil
			}
		}

		controllerutil.RemoveFinalizer(domain, finalizerName)
		if err := r.Update(ctx, domain); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DomainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Domain{}).
		Complete(r)
}
