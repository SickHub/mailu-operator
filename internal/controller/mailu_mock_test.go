package controller_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	"github.com/sickhub/mailu-operator/pkg/mailu"
)

func mailuMock() string {
	mux := chi.NewMux()

	first := true

	mux.Get("/domain", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)
		response := []mailu.DomainDetails{
			{
				Name: "example.com",
			},
			{
				Name: "foo.example.com",
			},
		}

		body, err := jsoniter.Marshal(response)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).NotTo(HaveOccurred())
	})

	mux.Get("/domain/example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		if first {
			first = false
			JSONError(w, http.StatusNotFound, "not found")
			return
		}

		domain := mailu.DomainDetails{
			Name:         "example.com",
			Alternatives: &[]string{"bar.example.com"},
		}
		body, err := jsoniter.Marshal(domain)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Get("/domain/nonexistent.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 404, "message": "Not found"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Post("/domain", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("post request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Patch("/domain/example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("patch request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Delete("/domain/example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("delete request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Get("/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)
		response := []mailu.User{
			{
				Email: "foo@example.com",
			},
			{
				Email: "bar@example.com",
			},
		}

		body, err := jsoniter.Marshal(response)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).NotTo(HaveOccurred())
	})

	mux.Get("/user/foo@example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		if first {
			first = false
			JSONError(w, http.StatusNotFound, "not found")
			return
		}

		domain := mailu.User{
			Email: "foo@example.com",
		}
		body, err := jsoniter.Marshal(domain)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Get("/user/foo@nonexistent.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 404, "message": "Not found"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Post("/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("post request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Patch("/user/foo@example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("patch request: %+v\n", r)
		//b, _ := io.ReadAll(r.Body)
		//defer r.Body.Close()
		//fmt.Printf("patch body: %s\n", string(b))

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Delete("/user/foo@example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("delete request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Get("/alias", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)
		response := []mailu.Alias{
			{
				Email: "foo@example.com",
			},
			{
				Email: "bar@example.com",
			},
		}

		body, err := jsoniter.Marshal(response)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).NotTo(HaveOccurred())
	})

	mux.Get("/alias/foo@example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		if first {
			first = false
			JSONError(w, http.StatusNotFound, "not found")
			return
		}

		domain := mailu.Alias{
			Email: "foo@example.com",
		}
		body, err := jsoniter.Marshal(domain)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Get("/alias/foo@nonexistent.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("get request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 404, "message": "Not found"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Post("/alias", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("post request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Patch("/alias/foo@example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("patch request: %+v\n", r)
		//b, _ := io.ReadAll(r.Body)
		//defer r.Body.Close()
		//fmt.Printf("patch body: %s\n", string(b))

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	mux.Delete("/alias/foo@example.com", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("delete request: %+v\n", r)

		body, err := jsoniter.Marshal(`{"code": 0, "message": "ok"}`)
		if err != nil {
			JSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		_, err = w.Write(body)
		Expect(err).ToNot(HaveOccurred())
	})

	httpSrv := httptest.NewServer(mux)
	//ginkgo.DeferCleanup(func() { httpSrv.Close() })

	return httpSrv.URL
}

// JSONError wraps an error json response.
func JSONError(rw http.ResponseWriter, c int, m string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(c)
	_, _ = rw.Write([]byte(fmt.Sprintf(`{"code": %d, "message": "%s"}`, c, m)))
}

// TODO: refactor to single functions
// TODO: use ghttp.Server with AppendHandlers
// WIP: done for alias

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

func prepareFindAlias(alias *operatorv1alpha1.Alias, status int) {
	mock.AppendHandlers(CombineHandlers(
		VerifyRequest("GET", "/alias/"+alias.Spec.Name+"@"+alias.Spec.Domain),
		getResponse(status),
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
