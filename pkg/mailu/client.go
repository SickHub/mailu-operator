package mailu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/oapi-codegen/runtime"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Alias defines model for Alias.
type Alias struct {
	// Comment a comment
	Comment     *string   `json:"comment,omitempty"`
	Destination *[]string `json:"destination,omitempty"`

	// Email the alias email address
	Email string `json:"email"`

	// Wildcard enable SQL Like wildcard syntax
	Wildcard *bool `json:"wildcard,omitempty"`
}

// Domain defines model for Domain.
type Domain struct {
	Alternatives *[]string `json:"alternatives,omitempty"`

	// Comment a comment
	Comment *string `json:"comment,omitempty"`

	// MaxAliases maximum number of aliases
	MaxAliases *int `json:"max_aliases,omitempty"`

	// MaxQuotaBytes maximum quota for mailbox
	MaxQuotaBytes *int `json:"max_quota_bytes,omitempty"`

	// MaxUsers maximum number of users
	MaxUsers *int `json:"max_users,omitempty"`

	// Name FQDN (e.g. example.com)
	Name string `json:"name"`

	// SignupEnabled allow signup
	SignupEnabled *bool `json:"signup_enabled,omitempty"`
}

type DomainDetails struct {
	Alternatives *[]string `json:"alternatives,omitempty"`

	// Comment a comment
	Comment *string `json:"comment,omitempty"`

	DNSAutoconfig *[]string `json:"dns_autoconfig,omitempty"`

	//DnsMX
	//DnsSPF
	//DnsDKIM
	//DnsDMARC
	//DnsDMARCReport
	//DnsTLSA

	// MaxAliases maximum number of aliases
	MaxAliases *int `json:"max_aliases,omitempty"`

	// MaxQuotaBytes maximum quota for mailbox
	MaxQuotaBytes *int `json:"max_quota_bytes,omitempty"`

	// MaxUsers maximum number of users
	MaxUsers *int `json:"max_users,omitempty"`

	// Managers lists managers of this domain
	Managers *[]string `json:"managers,omitempty"`

	// Name FQDN (e.g. example.com)
	Name string `json:"name"`

	// SignupEnabled allow signup
	SignupEnabled *bool `json:"signup_enabled,omitempty"`
}

// User defines model for UserGet.
type User struct {
	// AllowSpoofing Allow the user to spoof the sender (send email as anyone)
	AllowSpoofing *bool `json:"allow_spoofing,omitempty"`

	// ChangePwNextLogin Force the user to change their password at next login
	ChangePwNextLogin *bool `json:"change_pw_next_login,omitempty"`

	// Comment A description for the user. This description is shown on the Users page
	Comment *string `json:"comment,omitempty"`

	// DisplayedName The display name of the user within the Admin GUI
	DisplayedName *string `json:"displayed_name,omitempty"`

	// Email The email address of the user
	Email *string `json:"email,omitempty"`

	// EnableImap Allow email retrieval via IMAP
	EnableImap *bool `json:"enable_imap,omitempty"`

	// EnablePop Allow email retrieval via POP3
	EnablePop *bool `json:"enable_pop,omitempty"`

	// Enabled Enable the user
	Enabled            *bool     `json:"enabled,omitempty"`
	ForwardDestination *[]string `json:"forward_destination,omitempty"`

	// ForwardEnabled Enable auto forwarding
	ForwardEnabled *bool `json:"forward_enabled,omitempty"`

	// ForwardKeep Keep a copy of the forwarded email in the inbox
	ForwardKeep *bool `json:"forward_keep,omitempty"`

	// GlobalAdmin Make the user a global administrator
	GlobalAdmin *bool `json:"global_admin,omitempty"`

	// Password Hash of the user's password
	Password *string `json:"password,omitempty"`

	// QuotaBytes The maximum quota for the user’s email box in bytes
	QuotaBytes *int `json:"quota_bytes,omitempty"`

	// QuotaBytesUsed The size of the user’s email box in bytes
	QuotaBytesUsed *int `json:"quota_bytes_used,omitempty"`

	// ReplyBody The body of the automatic reply email
	ReplyBody *string `json:"reply_body,omitempty"`

	// ReplyEnabled Enable automatic replies. This is also known as out of office (ooo) or out of facility (oof) replies
	ReplyEnabled *bool `json:"reply_enabled,omitempty"`

	// ReplyEnddate End date for automatic replies in YYYY-MM-DD format.
	ReplyEnddate *openapi_types.Date `json:"reply_enddate,omitempty"`

	// ReplyStartdate Start date for automatic replies in YYYY-MM-DD format.
	ReplyStartdate *openapi_types.Date `json:"reply_startdate,omitempty"`

	// ReplySubject Optional subject for the automatic reply
	ReplySubject *string `json:"reply_subject,omitempty"`

	// SpamEnabled Enable the spam filter
	SpamEnabled *bool `json:"spam_enabled,omitempty"`

	// SpamMarkAsRead Enable marking spam mails as read
	SpamMarkAsRead *bool `json:"spam_mark_as_read,omitempty"`

	// SpamThreshold The user defined spam filter tolerance
	SpamThreshold *int `json:"spam_threshold,omitempty"`
}

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Doer performs HTTP requests.
//
// The standard http.Client implements this interface.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client which conforms to the OpenAPI3 specification for this service.
type Client struct {
	// The endpoint of the server conforming to this interface, with scheme,
	// https://api.deepmap.com for example. This can contain a path relative
	// to the server, such as https://api.deepmap.com/dev-test, and all the
	// paths in the swagger spec will be appended to the server.
	Server string

	// Doer for performing requests, typically a *http.Client with any
	// customized settings, such as certificate chains.
	Client HttpRequestDoer

	// A list of callbacks for modifying requests which are generated before sending over
	// the network.
	RequestEditors []RequestEditorFn
}

// ClientOption allows setting custom parameters during construction
type ClientOption func(*Client) error

// Creates a new Client, with reasonable defaults
func NewClient(server string, opts ...ClientOption) (*Client, error) {
	// create a client with sane default values
	client := Client{
		Server: server,
	}
	// mutate client and add all optional params
	for _, o := range opts {
		if err := o(&client); err != nil {
			return nil, err
		}
	}
	// ensure the server URL always has a trailing slash
	if !strings.HasSuffix(client.Server, "/") {
		client.Server += "/"
	}
	// create httpClient, if not already present
	if client.Client == nil {
		client.Client = &http.Client{}
	}
	return &client, nil
}

// WithHTTPClient allows overriding the default Doer, which is
// automatically created using http.Client. This is useful for tests.
func WithHTTPClient(doer HttpRequestDoer) ClientOption {
	return func(c *Client) error {
		c.Client = doer
		return nil
	}
}

// WithRequestEditorFn allows setting up a callback function, which will be
// called right before sending the request. This can be used to mutate the request.
func WithRequestEditorFn(fn RequestEditorFn) ClientOption {
	return func(c *Client) error {
		c.RequestEditors = append(c.RequestEditors, fn)
		return nil
	}
}

func (c *Client) FindDomain(ctx context.Context, domain string, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewFindDomainRequest(c.Server, domain)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) CreateDomain(ctx context.Context, body Domain, reqEditors ...RequestEditorFn) (*http.Response, error) {
	req, err := NewCreateDomainRequest(c.Server, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// UpdateDomain updates the domain with the given body.
func (c *Client) UpdateDomain(ctx context.Context, domain string, body Domain, reqEditors ...RequestEditorFn) (*http.Response, error) { //nolint:lll
	req, err := NewUpdateDomainRequest(c.Server, domain, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) DeleteDomain(ctx context.Context, domain string, reqEditors ...RequestEditorFn) (*http.Response, error) { //nolint:lll
	req, err := NewDeleteDomainRequest(c.Server, domain)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

// NewFindDomainRequest generates requests for FindDomain
func NewFindDomainRequest(server string, domain string) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "domain", runtime.ParamLocationPath, domain)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/domain/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// NewCreateDomainRequest calls the generic CreateDomain builder with application/json body
func NewCreateDomainRequest(server string, body Domain) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewCreateDomainRequestWithBody(server, "application/json", bodyReader)
}

// NewCreateDomainRequestWithBody generates requests for CreateDomain with any type of body
func NewCreateDomainRequestWithBody(server string, contentType string, body io.Reader) (*http.Request, error) {
	var err error

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := "/domain"
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewUpdateDomainRequest calls the generic UpdateDomain builder with application/json body
func NewUpdateDomainRequest(server string, domain string, body Domain) (*http.Request, error) {
	var bodyReader io.Reader
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	bodyReader = bytes.NewReader(buf)
	return NewUpdateDomainRequestWithBody(server, domain, "application/json", bodyReader)
}

// NewUpdateDomainRequestWithBody generates requests for UpdateDomain with any type of body
func NewUpdateDomainRequestWithBody(server string, domain string, contentType string, body io.Reader) (*http.Request, error) { //nolint:lll
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "domain", runtime.ParamLocationPath, domain)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/domain/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)

	return req, nil
}

// NewDeleteDomainRequest generates requests for DeleteDomain
func NewDeleteDomainRequest(server string, domain string) (*http.Request, error) {
	var err error

	var pathParam0 string

	pathParam0, err = runtime.StyleParamWithLocation("simple", false, "domain", runtime.ParamLocationPath, domain)
	if err != nil {
		return nil, err
	}

	serverURL, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	operationPath := fmt.Sprintf("/domain/%s", pathParam0)
	if operationPath[0] == '/' {
		operationPath = "." + operationPath
	}

	queryURL, err := serverURL.Parse(operationPath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("DELETE", queryURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *Client) applyEditors(ctx context.Context, req *http.Request, additionalEditors []RequestEditorFn) error {
	for _, r := range c.RequestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	for _, r := range additionalEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// JSONError wraps an error json response.
func JSONError(rw http.ResponseWriter, c int, m string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(c)
	_, _ = rw.Write([]byte(fmt.Sprintf(`{"errors": [{"title": "error", "detail": "%s"}]}`, m)))
}
