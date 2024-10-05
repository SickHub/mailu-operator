package controller_test

import (
	"net/http"

	. "github.com/onsi/gomega/ghttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	"github.com/sickhub/mailu-operator/pkg/mailu"
)

const (
	mockName    = "foo"
	mockDomain  = "example.com"
	mockComment = "some comment"
)

var (
	ResponseOK                 = RespondWith(http.StatusOK, `{"code": 0, "message": "ok"}`)
	ResponseBadRequest         = RespondWith(http.StatusBadRequest, `{"code": 400, "message": "bad request"}`)
	ResponseNotFound           = RespondWith(http.StatusNotFound, `{"code": 404, "message": "not found"}`)
	ResponseInternalError      = RespondWith(http.StatusInternalServerError, `{"code": 500, "message": "internal server error"}`)
	ResponseUnauthorized       = RespondWith(http.StatusUnauthorized, `{"code": 401, "message": "unauthorized"}`)
	ResponseForbidden          = RespondWith(http.StatusForbidden, `{"code": 402, "message": "forbidden"}`)
	ResponseConflict           = RespondWith(http.StatusConflict, `{"code": 409, "message": "conflict"}`)
	ResponseServiceUnavailable = RespondWith(http.StatusServiceUnavailable, `{"code": 503, "message": "service unavailable"}`)
)

func CreateResource(obj interface{}, name, domain string) client.Object {
	switch obj.(type) {
	case operatorv1alpha1.Alias:
		return &operatorv1alpha1.Alias{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       operatorv1alpha1.AliasSpec{Name: name, Domain: domain},
		}
	case operatorv1alpha1.User:
		return &operatorv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       operatorv1alpha1.UserSpec{Name: name, Domain: domain, RawPassword: "foo"},
		}
	case operatorv1alpha1.Domain:
		return &operatorv1alpha1.Domain{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       operatorv1alpha1.DomainSpec{Name: domain},
		}
	default:
		return nil
	}
}

func getResponse(status int) http.HandlerFunc {
	switch status {
	case http.StatusForbidden:
		return ResponseForbidden
	case http.StatusUnauthorized:
		return ResponseUnauthorized
	case http.StatusBadRequest:
		return ResponseBadRequest
	case http.StatusInternalServerError:
		return ResponseInternalError
	case http.StatusNotFound:
		return ResponseNotFound
	case http.StatusConflict:
		return ResponseConflict
	case http.StatusServiceUnavailable:
		return ResponseServiceUnavailable
	case http.StatusOK:
		fallthrough
	default:
		return ResponseOK
	}
}

// Alias
func prepareFindAlias(alias *operatorv1alpha1.Alias, status int) {
	response := getResponse(status)
	if status == http.StatusOK {
		response = RespondWithJSONEncoded(http.StatusOK, mailu.Alias{
			Email:       alias.Spec.Name + "@" + alias.Spec.Domain,
			Comment:     &alias.Spec.Comment,
			Destination: &alias.Spec.Destination,
			Wildcard:    &alias.Spec.Wildcard,
		})
	}
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("GET", "/alias/"+alias.Spec.Name+"@"+alias.Spec.Domain),
		response,
	))
}

func prepareCreateAlias(alias *operatorv1alpha1.Alias, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("POST", "/alias"),
		VerifyJSONRepresenting(mailu.Alias{
			Email:       alias.Spec.Name + "@" + alias.Spec.Domain,
			Comment:     &alias.Spec.Comment,
			Destination: &alias.Spec.Destination,
			Wildcard:    &alias.Spec.Wildcard,
		}),
		getResponse(status),
	))
}

func preparePatchAlias(alias *operatorv1alpha1.Alias, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("PATCH", "/alias/"+alias.Spec.Name+"@"+alias.Spec.Domain),
		getResponse(status),
	))
}

func prepareDeleteAlias(alias *operatorv1alpha1.Alias, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("DELETE", "/alias/"+alias.Spec.Name+"@"+alias.Spec.Domain),
		getResponse(status),
	))
}

// Domain
func prepareFindDomain(domain *operatorv1alpha1.Domain, status int) {
	response := getResponse(status)
	if status == http.StatusOK {
		response = RespondWithJSONEncoded(http.StatusOK, mailu.Domain{
			Name:          domain.Spec.Name,
			Alternatives:  &domain.Spec.Alternatives,
			Comment:       &domain.Spec.Comment,
			MaxAliases:    &domain.Spec.MaxAliases,
			MaxQuotaBytes: &domain.Spec.MaxQuotaBytes,
			MaxUsers:      &domain.Spec.MaxUsers,
			SignupEnabled: &domain.Spec.SignupEnabled,
		})
	}
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("GET", "/domain/"+domain.Spec.Name),
		response,
	))
}

func prepareCreateDomain(domain *operatorv1alpha1.Domain, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("POST", "/domain"),
		VerifyJSONRepresenting(mailu.Domain{
			Name:          domain.Spec.Name,
			Comment:       &domain.Spec.Comment,
			MaxAliases:    &domain.Spec.MaxAliases,
			MaxQuotaBytes: &domain.Spec.MaxQuotaBytes,
			MaxUsers:      &domain.Spec.MaxUsers,
			SignupEnabled: &domain.Spec.SignupEnabled,
		}),
		getResponse(status),
	))
}

func preparePatchDomain(domain *operatorv1alpha1.Domain, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("PATCH", "/domain/"+domain.Spec.Name),
		getResponse(status),
	))
}

func prepareDeleteDomain(domain *operatorv1alpha1.Domain, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("DELETE", "/domain/"+domain.Spec.Name),
		getResponse(status),
	))
}

// User
func prepareFindUser(user *operatorv1alpha1.User, status int) {
	response := getResponse(status)
	if status == http.StatusOK {
		response = RespondWithJSONEncoded(http.StatusOK, mailu.User{
			AllowSpoofing:      &user.Spec.AllowSpoofing,
			ChangePwNextLogin:  &user.Spec.ChangePassword,
			Comment:            &user.Spec.Comment,
			DisplayedName:      &user.Spec.DisplayedName,
			Email:              user.Spec.Name + "@" + user.Spec.Domain,
			Enabled:            &user.Spec.Enabled,
			EnableImap:         &user.Spec.EnableIMAP,
			EnablePop:          &user.Spec.EnablePOP,
			ForwardDestination: &user.Spec.ForwardDestination,
			ForwardEnabled:     &user.Spec.ForwardEnabled,
			ForwardKeep:        &user.Spec.ForwardKeep,
			GlobalAdmin:        &user.Spec.GlobalAdmin,
			Password:           &user.Spec.Name,
			QuotaBytes:         &user.Spec.QuotaBytes,
			QuotaBytesUsed:     &user.Spec.QuotaBytes,
			RawPassword:        &user.Spec.Name,
			ReplyBody:          &user.Spec.ReplyBody,
			ReplyEnabled:       &user.Spec.ReplyEnabled,
			ReplySubject:       &user.Spec.ReplySubject,
			SpamEnabled:        &user.Spec.SpamEnabled,
			SpamMarkAsRead:     &user.Spec.SpamMarkAsRead,
			SpamThreshold:      &user.Spec.SpamThreshold,
		})
	}
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("GET", "/user/"+user.Spec.Name+"@"+user.Spec.Domain),
		response,
	))
}

func prepareCreateUser(user *operatorv1alpha1.User, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("POST", "/user"),
		VerifyJSONRepresenting(mailu.User{
			Email:              user.Spec.Name + "@" + user.Spec.Domain,
			AllowSpoofing:      &user.Spec.AllowSpoofing,
			ChangePwNextLogin:  &user.Spec.ChangePassword,
			Comment:            &user.Spec.Comment,
			DisplayedName:      &user.Spec.DisplayedName,
			EnableImap:         &user.Spec.EnableIMAP,
			EnablePop:          &user.Spec.EnablePOP,
			Enabled:            &user.Spec.Enabled,
			ForwardDestination: &user.Spec.ForwardDestination,
			ForwardEnabled:     &user.Spec.ForwardEnabled,
			ForwardKeep:        &user.Spec.ForwardKeep,
			GlobalAdmin:        &user.Spec.GlobalAdmin,
			QuotaBytes:         &user.Spec.QuotaBytes,
			RawPassword:        &user.Spec.RawPassword,
			ReplyBody:          &user.Spec.ReplyBody,
			ReplyEnabled:       &user.Spec.ReplyEnabled,
			ReplySubject:       &user.Spec.ReplySubject,
			SpamEnabled:        &user.Spec.SpamEnabled,
			SpamMarkAsRead:     &user.Spec.SpamMarkAsRead,
			SpamThreshold:      &user.Spec.SpamThreshold,
		}),
		getResponse(status),
	))
}

func preparePatchUser(user *operatorv1alpha1.User, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("PATCH", "/user/"+user.Spec.Name+"@"+user.Spec.Domain),
		getResponse(status),
	))
}

func prepareDeleteUser(user *operatorv1alpha1.User, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("DELETE", "/user/"+user.Spec.Name+"@"+user.Spec.Domain),
		getResponse(status),
	))
}
