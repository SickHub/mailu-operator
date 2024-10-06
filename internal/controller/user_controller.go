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

	openapitypes "github.com/oapi-codegen/runtime/types"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	"github.com/sickhub/mailu-operator/pkg/mailu"
)

const (
	UserConditionTypeReady = "UserReady"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ApiURL    string
	ApiToken  string
	ApiClient *mailu.Client
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
func (r *UserReconciler) Reconcile(ctx context.Context, user *operatorv1alpha1.User) (ctrl.Result, error) {
	logr := log.FromContext(ctx, "user", user.Name)

	userOriginal := user.DeepCopy()

	// apply patches at the end, before returning
	defer func() {
		if err := r.Client.Patch(ctx, user.DeepCopy(), client.MergeFrom(userOriginal)); err != nil {
			logr.Error(err, "failed to patch resource")
		}
		if err := r.Client.Status().Patch(ctx, user.DeepCopy(), client.MergeFrom(userOriginal)); err != nil {
			logr.Error(err, "failed to patch resource status")
		}
	}()

	if user.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(user, FinalizerName) {
		controllerutil.AddFinalizer(user, FinalizerName)
	}

	result, err := r.reconcile(ctx, user)
	if err != nil {
		return result, err
	}

	if userOriginal.DeletionTimestamp != nil && !result.Requeue {
		controllerutil.RemoveFinalizer(user, FinalizerName)
	}

	return result, nil
}

func (r *UserReconciler) reconcile(ctx context.Context, user *operatorv1alpha1.User) (ctrl.Result, error) {
	logr := log.FromContext(ctx, "user", user.Name)

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

	foundUser, retry, err := r.getUser(ctx, user)
	if err != nil {
		if retry {
			logr.Info(fmt.Errorf("failed to get user, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		// we explicitly set the error in the status only on a permanent (non-retryable) error
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		logr.Error(err, "failed to get user")
		return ctrl.Result{}, nil
	}

	if user.DeletionTimestamp != nil {
		if foundUser == nil {
			// no need to delete it, if it does not exist
			return ctrl.Result{}, nil
		}
		return r.delete(ctx, user)
	}

	if foundUser == nil {
		return r.create(ctx, user)
	}

	return r.update(ctx, user, foundUser)
}

func (r *UserReconciler) create(ctx context.Context, user *operatorv1alpha1.User) (ctrl.Result, error) {
	logr := log.FromContext(ctx, "namespace", user.Namespace, "user", user.Name)

	retry, err := r.createUser(ctx, user)
	if err != nil {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		if retry {
			logr.Info(fmt.Errorf("failed to create user, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to create user")
		return ctrl.Result{}, err
	}

	if !retry {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionTrue, "Created", "User created in MailU"))
		logr.Info("created user")
	}

	return ctrl.Result{Requeue: retry}, nil
}

func (r *UserReconciler) update(ctx context.Context, user *operatorv1alpha1.User, apiUser *mailu.User) (ctrl.Result, error) {
	logr := log.FromContext(ctx, "namespace", user.Namespace, "user", user.Name)

	newUser, err := r.userFromSpec(user.Spec)
	if err != nil {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		logr.Error(err, "failed to get user from spec")
		return ctrl.Result{}, err
	}

	// reset some values that should not be updated
	newUser.RawPassword = nil
	apiUser.Password = nil
	apiUser.QuotaBytesUsed = nil

	jsonNew, _ := json.Marshal(newUser) //nolint:errcheck
	jsonOld, _ := json.Marshal(apiUser) //nolint:errcheck

	if reflect.DeepEqual(jsonNew, jsonOld) {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionTrue, "Updated", "User updated in MailU"))
		return ctrl.Result{}, nil
	}

	retry, err := r.updateUser(ctx, newUser)
	if err != nil {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		if retry {
			logr.Info(fmt.Errorf("failed to update user, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to update user")
		return ctrl.Result{}, err
	}

	if !retry {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionTrue, "Updated", "User updated in MailU"))
		logr.Info("updated user")
	}

	return ctrl.Result{Requeue: retry}, nil
}

func (r *UserReconciler) delete(ctx context.Context, user *operatorv1alpha1.User) (ctrl.Result, error) {
	logr := log.FromContext(ctx, "namespace", user.Namespace, "user", user.Name)

	retry, err := r.deleteUser(ctx, user)
	if err != nil {
		meta.SetStatusCondition(&user.Status.Conditions, getUserReadyCondition(metav1.ConditionFalse, "Error", err.Error()))
		if retry {
			logr.Info(fmt.Errorf("failed to delete user, requeueing: %w", err).Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
		}
		logr.Error(err, "failed to delete user")
		return ctrl.Result{}, err
	}

	if !retry {
		logr.Info("deleted user")
	}

	return ctrl.Result{Requeue: retry}, nil
}

func (r *UserReconciler) getUser(ctx context.Context, user *operatorv1alpha1.User) (*mailu.User, bool, error) {
	found, err := r.ApiClient.FindUser(ctx, user.Spec.Name+"@"+user.Spec.Domain)
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
		foundUser := &mailu.User{}
		err = json.Unmarshal(body, &foundUser)
		if err != nil {
			return nil, true, err
		}

		return foundUser, false, nil
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

func (r *UserReconciler) createUser(ctx context.Context, user *operatorv1alpha1.User) (bool, error) {
	logr := log.FromContext(ctx, "user", user.Name)
	email := user.Spec.Name + "@" + user.Spec.Domain

	// raw password is required during creation
	if user.Spec.RawPassword == "" {
		var err error
		if user.Spec.PasswordSecret != "" && user.Spec.PasswordKey != "" {
			user.Spec.RawPassword, err = r.getUserPassword(ctx, user.Namespace, user.Spec.PasswordSecret, user.Spec.PasswordKey)
			if err != nil {
				logr.Error(err, fmt.Sprintf("failed to get password from secret %s/%s", user.Namespace, user.Spec.PasswordSecret))
				return true, err
			}
			logr.Info(fmt.Sprintf("using password from secret for user %s", email))
		} else {
			// initial random password if none given
			user.Spec.RawPassword, err = password.Generate(20, 2, 2, false, false)
			if err != nil {
				logr.Error(err, fmt.Sprintf("failed to generate password for user %s", email))
				return true, err
			}
			logr.Info(fmt.Sprintf("using generated password for user %s", email))
		}
	}

	newUser, err := r.userFromSpec(user.Spec)
	if err != nil {
		return false, err
	}

	res, err := r.ApiClient.CreateUser(ctx, newUser)
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

func (r *UserReconciler) updateUser(ctx context.Context, newUser mailu.User) (bool, error) {
	res, err := r.ApiClient.UpdateUser(ctx, newUser.Email, newUser)
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

func (r *UserReconciler) deleteUser(ctx context.Context, user *operatorv1alpha1.User) (bool, error) {
	res, err := r.ApiClient.DeleteUser(ctx, user.Spec.Name+"@"+user.Spec.Domain)
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
		RawPassword:        &spec.RawPassword,
		ReplyBody:          &spec.ReplyBody,
		ReplyEnabled:       &spec.ReplyEnabled,
		ReplySubject:       &spec.ReplySubject,
		SpamEnabled:        &spec.SpamEnabled,
		SpamMarkAsRead:     &spec.SpamMarkAsRead,
		SpamThreshold:      &spec.SpamThreshold,
	}

	// mimick API behaviour
	if spec.ForwardDestination == nil {
		u.ForwardDestination = &[]string{}
	}

	// convert Dates if set
	if spec.ReplyStartDate != "" {
		d := &openapitypes.Date{}
		err := d.UnmarshalText([]byte(spec.ReplyStartDate))
		if err != nil {
			return mailu.User{}, err
		}
		u.ReplyStartDate = d
	}
	if spec.ReplyEndDate != "" {
		d := &openapitypes.Date{}
		err := d.UnmarshalText([]byte(spec.ReplyEndDate))
		if err != nil {
			return mailu.User{}, err
		}
		u.ReplyEndDate = d
	}

	return u, nil
}

func (r *UserReconciler) getUserPassword(ctx context.Context, namespace, secret, key string) (string, error) {
	s := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secret, Namespace: namespace}, s)
	if err != nil {
		return "", err
	}
	return string(s.Data[key]), nil
}

func getUserReadyCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    UserConditionTypeReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.User{}).
		Complete(reconcile.AsReconciler(r.Client, r))
}
