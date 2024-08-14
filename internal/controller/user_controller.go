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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/types"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"gitlab.rootcrew.net/rootcrew/services/mailu-operator/pkg/mailu"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "gitlab.rootcrew.net/rootcrew/services/mailu-operator/api/v1alpha1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ApiURL   string
	ApiToken string
}

//+kubebuilder:rbac:groups=operator.mailu.io,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.mailu.io,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.mailu.io,resources=users/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:gocyclo
	logr := log.FromContext(ctx)

	user := &operatorv1alpha1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	api, err := mailu.NewClient(r.ApiURL, mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+r.ApiToken)
		return nil
	}), mailu.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		body := []byte{}
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

	email := user.Spec.Name + "@" + user.Spec.Domain

	find, err := api.FindUser(ctx, email)
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

	if user.ObjectMeta.DeletionTimestamp.IsZero() {
		// Add a finalizer if not present
		if !controllerutil.ContainsFinalizer(user, finalizerName) {
			user.ObjectMeta.Finalizers = append(user.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, user); err != nil {
				//log.Error(err, "unable to update Tenant")
				return ctrl.Result{Requeue: true}, err
			}
		}

		var response *http.Response
		switch find.StatusCode {
		case http.StatusNotFound:
			newUser, err := r.userFromSpec(user.Spec)
			if err != nil {
				logr.Error(err, fmt.Sprintf("failed create user from spec, invalid date: %s or %s", user.Spec.ReplyStartDate, user.Spec.ReplyEndDate))
				return ctrl.Result{}, err
			}

			// set the password
			rawPassword := user.Spec.RawPassword
			if rawPassword == "" && user.Spec.PasswordSecret != "" && user.Spec.PasswordKey != "" {
				rawPassword, err = r.getUserPassword(ctx, req.Namespace, user.Spec.PasswordSecret, user.Spec.PasswordKey)
				if err != nil {
					logr.Error(err, fmt.Sprintf("failed to get password from secret %s/%s", req.Namespace, user.Spec.PasswordSecret))
					return ctrl.Result{Requeue: true}, err
				}
			}

			if rawPassword != "" {
				newUser.RawPassword = &rawPassword
			}

			response, err = api.CreateUser(ctx, newUser)
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
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionFalse, Reason: "Created", Message: "User creation failed: " + string(body)})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Error(err, fmt.Sprintf("failed to update user: %d %s", response.StatusCode, body))
				return ctrl.Result{Requeue: false}, nil
			case http.StatusConflict:
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionFalse, Reason: "Created", Message: "User creation failed: " + string(body)})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Error(err, fmt.Sprintf("failed to update user: %d %s", response.StatusCode, body))
				return ctrl.Result{Requeue: false}, nil
			case http.StatusOK:
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionTrue, Reason: "Created", Message: "User created"})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Info(fmt.Sprintf("user %s created", email))
				return ctrl.Result{}, nil
			default:
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionFalse, Reason: "Created", Message: "User creation failed: " + string(body)})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to create user: %d %s", response.StatusCode, body)
			}

		case http.StatusOK:
			body, err := io.ReadAll(find.Body)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			foundUser := mailu.User{}
			err = json.Unmarshal(body, &foundUser)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			newUser, err := r.userFromSpec(user.Spec)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			// reset some values that don't exist in new user or should not be updated
			foundUser.Password = nil
			foundUser.QuotaBytesUsed = nil

			// ignore Dates if not set
			if newUser.ReplyStartDate == nil {
				foundUser.ReplyStartDate = nil
			}
			if newUser.ReplyEndDate == nil {
				foundUser.ReplyEndDate = nil
			}

			// no change
			if reflect.DeepEqual(foundUser, newUser) {
				return ctrl.Result{}, nil
			}

			response, err = api.UpdateUser(ctx, email, newUser)
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
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionFalse, Reason: "Updated", Message: "User update failed: " + string(body)})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Error(err, fmt.Sprintf("failed to update user: %d %s", response.StatusCode, body))
				return ctrl.Result{Requeue: false}, nil
			case http.StatusOK:
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionTrue, Reason: "Updated", Message: "User updated"})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				logr.Info(fmt.Sprintf("user %s updated", email))
				return ctrl.Result{}, nil
			default:
				meta.SetStatusCondition(&user.Status.Conditions, metav1.Condition{Type: "Available", Status: metav1.ConditionFalse, Reason: "Updated", Message: "User update failed: " + string(body)})
				err = r.Status().Update(ctx, user)
				if err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				return ctrl.Result{Requeue: true}, fmt.Errorf("failed to update user: %d %s", response.StatusCode, body)
			}

		default:
			err = errors.New("unexpected status code")
			logr.Error(err, fmt.Sprintf("unexpected status code: %d", find.StatusCode))
			return ctrl.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(user, finalizerName) {
		// Domain removed -> delete
		if find.StatusCode != http.StatusNotFound {
			res, err := api.DeleteUser(ctx, user.Spec.Name+"@"+user.Spec.Domain)
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			if res.StatusCode != http.StatusOK {
				return ctrl.Result{Requeue: true}, nil
			}
		}

		controllerutil.RemoveFinalizer(user, finalizerName)
		if err := r.Update(ctx, user); err != nil {
			return ctrl.Result{}, err
		}
		logr.Info(fmt.Sprintf("user %s deleted", email))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.User{}).
		Complete(r)
}

func (r *UserReconciler) getUserPassword(ctx context.Context, namespace, secret, key string) (string, error) {
	s := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secret, Namespace: namespace}, s)
	if err != nil {
		return "", err
	}
	return string(s.Data[key]), nil
}

func (r *UserReconciler) userFromSpec(spec operatorv1alpha1.UserSpec) (mailu.User, error) {
	u := mailu.User{
		Email:              spec.Name + "@" + spec.Domain,
		AllowSpoofing:      &spec.AllowSpoofing,
		ChangePwNextLogin:  &spec.ChangePassword,
		Comment:            &spec.Comment,
		DisplayedName:      &spec.DisplayedName,
		Enabled:            &spec.Enabled,
		EnableImap:         &spec.EnableIMAP,
		EnablePop:          &spec.EnablePOP,
		ForwardDestination: &spec.ForwardDestination,
		ForwardEnabled:     &spec.ForwardEnabled,
		ForwardKeep:        &spec.ForwardKeep,
		GlobalAdmin:        &spec.GlobalAdmin,
		QuotaBytes:         &spec.QuotaBytes,
		ReplyBody:          &spec.ReplyBody,
		ReplyEnabled:       &spec.ReplyEnabled,
		ReplySubject:       &spec.ReplySubject,
		SpamEnabled:        &spec.SpamEnabled,
		SpamMarkAsRead:     &spec.SpamMarkAsRead,
		SpamThreshold:      &spec.SpamThreshold,
	}

	// convert Dates if set
	if spec.ReplyStartDate != "" {
		d := &openapi_types.Date{}
		err := d.UnmarshalText([]byte(spec.ReplyStartDate))
		if err != nil {
			return mailu.User{}, err
		}
		u.ReplyStartDate = d
	}
	if spec.ReplyEndDate != "" {
		d := &openapi_types.Date{}
		err := d.UnmarshalText([]byte(spec.ReplyEndDate))
		if err != nil {
			return mailu.User{}, err
		}
		u.ReplyEndDate = d
	}

	return u, nil
}
