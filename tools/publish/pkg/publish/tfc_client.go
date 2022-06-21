package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// TFCUploadReq is a collection of provider upload request
type TFCUploadReq struct {
	DistDir         string
	BinaryRename    string
	BinaryName      string
	ProviderName    string
	GPGKeyID        string
	GPGIdentityName string
	TFCOrg          string
	TFCToken        string
}

// TFCClient is a collection of http client request params
type TFCClient struct {
	token   string
	baseURL string
	org     string
	log     *zap.SugaredLogger
}

// TFCClientOpt takes TFC client options
type TFCClientOpt func(*TFCClient)

// NewTFCClient creates and returns a new instance of TFCClient
func NewTFCClient(opts ...TFCClientOpt) (*TFCClient, error) {
	client := &TFCClient{
		log: zap.NewExample().Sugar(),
	}

	for _, opt := range opts {
		opt(client)
	}

	_, err := url.Parse(client.baseURL)
	if err != nil {
		return client, err
	}

	if client.token == "" {
		return client, fmt.Errorf("you must supply a token")
	}

	if client.org == "" {
		return client, fmt.Errorf("you must supply an org")
	}

	return client, nil
}

// WithTFCLog sets the logging
func WithTFCLog(log *zap.SugaredLogger) TFCClientOpt {
	return func(client *TFCClient) {
		client.log = log
	}
}

// WithTFCOrg sets the base url
func WithTFCOrg(org string) TFCClientOpt {
	return func(client *TFCClient) {
		b := &url.URL{
			Scheme: "https",
			Host:   "app.terraform.io",
			Path:   filepath.Join("api/v2/organizations", org),
		}
		client.baseURL = b.String()
		client.org = org
	}
}

// WithTFCToken sets the TFC token
func WithTFCToken(token string) TFCClientOpt {
	return func(client *TFCClient) { client.token = token }
}

// TFCProvider is a collection of TFC provider registry namespace
type TFCProvider struct {
	Name         string
	Namespace    string
	RegistryType string
	Org          string
	URL          string
}

// TFCProviderVersion is a collection of TFC provider's version
type TFCProviderVersion struct {
	Version        string
	RegistryType   string
	KeyID          string
	Org            string
	SHAUploaded    bool
	SHASigUploaded bool
	SHASumsURL     string
	SHASumsSigURL  string
}

// TFCProviderPlatform struct is a collection of provider platform data
type TFCProviderPlatform struct {
	PlatformID             string
	Version                string
	RegistryType           string
	Org                    string
	OS                     string
	Arch                   string
	Filename               string
	Filepath               string
	SHAsum                 string
	PlatformBinaryUploaded bool
	PlatformBinaryURL      string
}

// errTFCAPI in an API reponse error
type errTFCAPI struct {
	err    error
	msg    string
	status int
	Errors []struct {
		Title  string `json:"title"`
		Status string `json:"status"`
	} `json:"errors"`
}

// Returns the TFCAPI's error title and status
func (e *errTFCAPI) String() string {
	msg := strings.Builder{}
	msg.WriteString(fmt.Sprintln(e.msg))
	if e.status != 0 {
		msg.WriteString(fmt.Sprintf("HTTP: %d", e.status))
	}
	if e.Errors != nil {
		for _, err := range e.Errors {
			msg.WriteString(fmt.Sprintf("%s: %s\n", err.Title, err.Status))
		}
	}

	return msg.String()
}

func (e *errTFCAPI) Error() string {
	return e.String()
}

func (e *errTFCAPI) Unwrap() error {
	return e.err
}

type errTFCAPIOpt func(*errTFCAPI)

func newTFCAPIError(msg string, opts ...errTFCAPIOpt) *errTFCAPI {
	err := &errTFCAPI{msg: msg}

	for _, opt := range opts {
		opt(err)
	}

	return err
}

func withErrTFCAPIResponse(res *http.Response) errTFCAPIOpt {
	return func(e *errTFCAPI) {
		e.status = res.StatusCode

		body, err := io.ReadAll(res.Body)
		if err != nil {
			// Swalling the error since we're returning an error
			return
		}

		_ = json.Unmarshal(body, err)
	}
}

func newCreateProviderReq() *createProviderReq {
	return &createProviderReq{
		Data: newProviderData(),
	}
}

func newCreateProviderRes() *createProviderRes {
	return &createProviderRes{
		Data: newProviderData(),
	}
}

func newCreateVersionReq() *createVersionReq {
	return &createVersionReq{
		Data: newProviderData(),
	}
}

func newCreateVersionRes() *createVersionRes {
	return &createVersionRes{
		Data: newProviderData(),
	}
}

func newFindProviderRes() *findProviderRes {
	return &findProviderRes{
		Data: newProviderData(),
	}
}

func newFindVersionRes() *findVersionRes {
	return &findVersionRes{
		Data: newProviderData(),
	}
}

func newProviderData() *providerData {
	return &providerData{
		Attributes: newProviderAttributes(),
		Links:      newProviderLinks(),
	}
}

func newProviderAttributes() *providerAttributes {
	return &providerAttributes{}
}

func newProviderLinks() *providerLinks {
	return &providerLinks{}
}

type createProviderReq struct {
	Data *providerData `json:"data"`
}

type createProviderRes struct {
	Data *providerData `json:"data"`
}

type findProviderRes struct {
	Data *providerData `json:"data"`
}

type createVersionReq struct {
	Data *providerData `json:"data"`
}

type createVersionRes struct {
	Data *providerData `json:"data"`
}

type findVersionRes struct {
	Data *providerData `json:"data"`
}

// providerData is a collection of provider's data payload
type providerData struct {
	Type       string              `json:"type,omitempty"`
	Attributes *providerAttributes `json:"attributes,omitempty"`
	Links      *providerLinks      `json:"links,omitempty"`
}

// providerAttributes is a collection of provider's attibutes
type providerAttributes struct {
	Name           string `json:"name,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	RegistryType   string `json:"registry-name,omitempty"`
	Version        string `json:"version,omitempty"`
	KeyID          string `json:"key-id,omitempty"`
	SHAUploaded    bool   `json:"shasums-uploaded,omitempty"`
	SHASigUploaded bool   `json:"shasums-sig-uploaded,omitempty"`
}

// providerLinks is a collection of provider's links
type providerLinks struct {
	Self               string `json:"self,omitempty"`
	ShasumsUpload      string `json:"shasums-upload,omitempty"`
	ShasumssigUpload   string `json:"shasums-sig-upload,omitempty"`
	ShasumsDownload    string `json:"shasums-download,omitempty"`
	ShasumssigDownload string `json:"shasums-sig-download,omitempty"`
}

func newProvidersPlatformData() *providersPlatformData {
	return &providersPlatformData{
		Data: []*platformData{},
	}
}

func newcreatePlatformReq() *createPlatformReq {
	return &createPlatformReq{
		Data: &createPlatformReqData{
			Attributes: &createPlatformReqDataAttrs{},
			Links:      &createPlatformReqDataLinks{},
		},
	}
}

// createPlatformReq is a data payload for create provider platform request
type createPlatformReq struct {
	Data *createPlatformReqData `json:"data,omitempty"`
}

// createPlatformReqData is a collection create provider platform's request data payload
type createPlatformReqData struct {
	PlatformID string                      `json:"id,omitempty"`
	Type       string                      `json:"type,omitempty"`
	Attributes *createPlatformReqDataAttrs `json:"attributes,omitempty"`
	Links      *createPlatformReqDataLinks `json:"links,omitempty"`
}

// createPlatformReqDataAttrs is a collection create provider platform's request data attributes
type createPlatformReqDataAttrs struct {
	OS             string `json:"os,omitempty"`
	Arch           string `json:"arch,omitempty"`
	SHAsum         string `json:"shasum,omitempty"`
	Filename       string `json:"filename,omitempty"`
	BinaryUploaded bool   `json:"provider-binary-uploaded,omitempty"`
}

// createPlatformReqDataLinks is a collection provider's platform binary URLs
type createPlatformReqDataLinks struct {
	ProviderBinaryDownload string `json:"provider-binary-download,omitempty"`
	ProviderBinaryUpload   string `json:"provider-binary-upload,omitempty"`
}

func newCreatePlatformRes() *createPlatformRes {
	return &createPlatformRes{
		Data: &createPlatformResData{
			Attributes: &createPlatformResAttrs{},
			Links:      &createPlatformResLinks{},
		},
	}
}

// createPlatformRes is a data payload response for create provider platform
type createPlatformRes struct {
	Data *createPlatformResData `json:"data,omitempty"`
}

// createPlatformResData is a collection of create provider platform's response data
type createPlatformResData struct {
	PlatformID string                  `json:"id,omitempty"`
	Type       string                  `json:"type,omitempty"`
	Attributes *createPlatformResAttrs `json:"attributes,omitempty"`
	Links      *createPlatformResLinks `json:"links,omitempty"`
}

// createPlatformResAttrs is a collection of create provider platform's response attributes
type createPlatformResAttrs struct {
	OS             string `json:"os,omitempty"`
	Arch           string `json:"arch,omitempty"`
	SHAsum         string `json:"shasum,omitempty"`
	Filename       string `json:"filename,omitempty"`
	BinaryUploaded bool   `json:"provider-binary-uploaded,omitempty"`
}

// createPlatformResLinks is a collection provider's platform binary URLs
type createPlatformResLinks struct {
	ProviderBinaryDownload string `json:"provider-binary-download,omitempty"`
	ProviderBinaryUpload   string `json:"provider-binary-upload,omitempty"`
}

type platformData struct {
	PlatformID string              `json:"id,omitempty"`
	Type       string              `json:"type,omitempty"`
	Attributes *platformAttributes `json:"attributes,omitempty"`
	Links      *platformLinks      `json:"links,omitempty"`
}

// providersPlatformData is a data payload for provider platform
type providersPlatformData struct {
	Data []*platformData `json:"data,omitempty"`
}

// platformAttributes is a collection provider's platform attributes
type platformAttributes struct {
	OS             string `json:"os,omitempty"`
	Arch           string `json:"arch,omitempty"`
	SHAsum         string `json:"shasum,omitempty"`
	Filename       string `json:"filename,omitempty"`
	BinaryUploaded bool   `json:"provider-binary-uploaded,omitempty"`
}

// platformLinks is a collection provider's platform binary URLs
type platformLinks struct {
	ProviderBinaryDownload string `json:"provider-binary-download,omitempty"`
	ProviderBinaryUpload   string `json:"provider-binary-upload,omitempty"`
}

// TFCRequest is a TFC API request
type TFCRequest struct {
	Scheme     string // "https"
	Host       string // "app.terraform.io"
	ReqPath    string
	HTTPMethod string
	Body       io.Reader
}

// TFCRequestOpt are functional options for a new Request
type TFCRequestOpt func(*TFCRequest) (*TFCRequest, error)

// WithRequestBody sets the request body
func WithRequestBody(body io.Reader) TFCRequestOpt {
	return func(req *TFCRequest) (*TFCRequest, error) {
		req.Body = body
		return req, nil
	}
}

// NewTFCRequest takes TFCRequestOpt args and returns a new TFC API request
func NewTFCRequest(opts ...TFCRequestOpt) (*TFCRequest, error) {
	tfcreq := &TFCRequest{
		Scheme: "https",
		Host:   "app.terraform.io",
	}

	for _, opt := range opts {
		tfcreq, err := opt(tfcreq)
		if err != nil {
			return tfcreq, err
		}
	}
	return tfcreq, nil
}

// DoRequest takes a context, http request params with an optional data body and returns an http response.
func (c *TFCClient) DoRequest(ctx context.Context, tfcReq *TFCRequest) (*http.Response, error) {
	u := &url.URL{
		Scheme: tfcReq.Scheme,  // "https"
		Host:   tfcReq.Host,    // "app.terraform.io"
		Path:   tfcReq.ReqPath, // filepath.Join("api/v2/organizations", c.org, "registry-providers/private", namespace, providerName)
	}
	reqURL := u.String()

	c.log.Infow(
		"TFC API request URL",
		"Url", reqURL,
	)

	req, _ := http.NewRequestWithContext(ctx, tfcReq.HTTPMethod, reqURL, tfcReq.Body)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/vnd.api+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, fmt.Errorf("error sending http request: %w", err)
	}
	return resp, nil
}

// FindOrCreateProvider - FindProvider - CreateProvider
func (c *TFCClient) FindOrCreateProvider(ctx context.Context, namespace string, providerName string) error {
	_, err := c.FindPrivateProvider(ctx, namespace, providerName)
	tfcErr := &errTFCAPI{}
	if errors.As(err, &tfcErr) {
		if tfcErr.status == http.StatusNotFound {
			c.log.Infow(
				"private provider not found",
				"provider name", providerName,
				"namespace", namespace,
				"URL", c.baseURL,
			)
			return c.CreatePrivateProvider(ctx, namespace, providerName)
		}
	}
	return err
}

// FindPrivateProvider searches for existing private provider
func (c *TFCClient) FindPrivateProvider(
	ctx context.Context,
	namespace string,
	providerName string,
) (*TFCProvider, error) {
	c.log.Infow(
		"searching private provider",
		"provider name", providerName,
		"namespace", namespace,
		"URL", c.baseURL,
	)
	provider := &TFCProvider{Org: c.org}
	req, _ := NewTFCRequest()
	req.ReqPath = fmt.Sprintf("api/v2/organizations/%s/registry-providers/private/%s/%s", c.org, namespace, providerName)
	req.HTTPMethod = "GET"

	//  https://app.terraform.io/api/v2/organizations/<ORG>/registry-providers/private/<NAMESPACE>/<NAME>"
	tfcAPIResp, err := c.DoRequest(ctx, req)
	if err != nil {
		return provider, err
	}
	defer tfcAPIResp.Body.Close()

	if tfcAPIResp.StatusCode != http.StatusOK {
		return provider, newTFCAPIError("provider search error", withErrTFCAPIResponse(tfcAPIResp))
	}

	body, err := io.ReadAll(tfcAPIResp.Body)
	if err != nil {
		return provider, fmt.Errorf("reading provider response: %w", err)
	}

	providerBody := newFindProviderRes()
	err = json.Unmarshal(body, &providerBody)
	if err != nil {
		return provider, fmt.Errorf("unmarshaling provider response: %w", err)
	}

	provider.Name = providerBody.Data.Attributes.Name
	provider.Namespace = providerBody.Data.Attributes.Namespace
	provider.URL = providerBody.Data.Links.Self

	c.log.Infow(
		"found private provider",
		"provider name", provider.Name,
		"namespace", provider.Namespace,
		"URL", provider.URL,
	)

	return provider, nil
}

// CreatePrivateProvider creates a private provider
func (c *TFCClient) CreatePrivateProvider(
	ctx context.Context,
	namespace string,
	providerName string,
) error {
	c.log.Infow(
		"creating private provider",
		"provider", providerName,
		"namespace", namespace,
		"URL", c.baseURL,
	)
	providerdata := newCreateProviderReq()
	providerdata.Data = &providerData{
		Type: "registry-providers",
		Attributes: &providerAttributes{
			Name:         providerName,
			Namespace:    namespace,
			RegistryType: "private",
		},
	}

	payloadBytes, err := json.Marshal(providerdata)
	if err != nil {
		return err
	}
	databody := bytes.NewReader(payloadBytes)

	req, _ := NewTFCRequest(WithRequestBody(databody))
	req.ReqPath = fmt.Sprintf("api/v2/organizations/%s/registry-providers", c.org)
	req.HTTPMethod = "POST"

	tfcAPIResp, err := c.DoRequest(ctx, req)
	if err != nil {
		return err
	}
	defer tfcAPIResp.Body.Close()

	if tfcAPIResp.StatusCode != http.StatusCreated {
		return newTFCAPIError("creating provider failed", withErrTFCAPIResponse(tfcAPIResp))
	}

	body, err := io.ReadAll(tfcAPIResp.Body)
	if err != nil {
		return fmt.Errorf("reading provider response: %w", err)
	}

	providerBody := newCreateProviderRes()
	err = json.Unmarshal(body, &providerBody)
	if err != nil {
		return fmt.Errorf("unmarshaling provider response: %w", err)
	}

	c.log.Infow(
		"created private provider",
		"provider name", providerBody.Data.Attributes.Name,
		"namespace", providerBody.Data.Attributes.Namespace,
		"URL", providerBody.Data.Links.Self,
	)
	return err
}

// FindOrCreateVersion - FindProviderVersion - CreateProviderVersion
func (c *TFCClient) FindOrCreateVersion(
	ctx context.Context,
	namespace string,
	providerName string,
	providerVersion string,
	gpgKeyID string,
	sha256sumsPath string,
) error {
	_, err := c.FindProviderVersion(ctx, namespace, providerName, providerVersion, sha256sumsPath)
	tfcErr := &errTFCAPI{}
	if errors.As(err, &tfcErr) {
		if tfcErr.status == http.StatusNotFound {
			return c.CreateProviderVersion(ctx, namespace, providerName, providerVersion, gpgKeyID, sha256sumsPath)
		}
	}
	return nil
}

// FindProviderVersion searches for existing private provider version
func (c *TFCClient) FindProviderVersion(
	ctx context.Context,
	namespace string,
	providerName string,
	providerVersion string,
	sha256sumsPath string,
) (*TFCProviderVersion, error) {
	c.log.Infow(
		"searching provider version",
		"version", providerVersion,
		"provider name", providerName,
		"namespace", namespace,
	)
	providerversion := &TFCProviderVersion{Version: providerVersion}
	req, _ := NewTFCRequest()
	req.ReqPath = fmt.Sprintf("api/v2/organizations/%s/registry-providers/private/%s/%s/versions/%s", c.org, namespace, providerName, providerVersion)
	req.HTTPMethod = "GET"

	//  https://app.terraform.io/api/v2/organizations/<ORG>/registry-providers/private/<NAMESPACE>/<NAME>/versions/<VERSION>"
	tfcAPIResp, err := c.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer tfcAPIResp.Body.Close()

	if tfcAPIResp.StatusCode != http.StatusOK {
		return providerversion, newTFCAPIError("error searching for providerversion version", withErrTFCAPIResponse(tfcAPIResp))
	}

	body, err := io.ReadAll(tfcAPIResp.Body)
	if err != nil {
		return providerversion, fmt.Errorf("reading provider version response: %w", err)
	}

	providerVersionBody := newFindVersionRes()
	err = json.Unmarshal(body, &providerVersionBody)
	if err != nil {
		return providerversion, fmt.Errorf("unmarshaling provider version response: %w", err)
	}

	providerversion.Version = providerVersionBody.Data.Attributes.Version
	providerversion.KeyID = providerVersionBody.Data.Attributes.KeyID
	providerversion.SHAUploaded = providerVersionBody.Data.Attributes.SHAUploaded
	providerversion.SHASigUploaded = providerVersionBody.Data.Attributes.SHASigUploaded
	c.log.Infow(
		"found provider version",
		"provider version", providerversion.Version,
		"provider version key", providerversion.KeyID,
		"provider sha sum uploaded?", providerversion.SHAUploaded,
		"provider sha sign uploaded?", providerversion.SHASigUploaded,
	)

	if !providerversion.SHAUploaded {
		providerversion.SHASumsURL = providerVersionBody.Data.Links.ShasumsUpload
		err = c.uploadFile(ctx, sha256sumsPath, providerversion.SHASumsURL)
		if err != nil {
			return providerversion, fmt.Errorf("failed to upload shasums file: %w", err)
		}
	} else {
		providerversion.SHASumsURL = providerVersionBody.Data.Links.ShasumsDownload
		c.log.Infow(
			"SHA256SUMS download",
			"link", providerversion.SHASumsURL,
		)
	}

	if !providerversion.SHASigUploaded {
		providerversion.SHASumsSigURL = providerVersionBody.Data.Links.ShasumssigUpload
		sha256sigPath := fmt.Sprintf("%s.sig", sha256sumsPath)
		err = c.uploadFile(ctx, sha256sigPath, providerversion.SHASumsSigURL)
		if err != nil {
			return providerversion, fmt.Errorf("failed to upload shasums sig file: %w", err)
		}
	} else {
		providerversion.SHASumsSigURL = providerVersionBody.Data.Links.ShasumssigDownload
		c.log.Infow(
			"SHA256SUMS.sig download",
			"link", providerversion.SHASumsURL,
		)
	}
	return providerversion, nil
}

// CreateProviderVersion creates a private provider version
func (c *TFCClient) CreateProviderVersion(
	ctx context.Context,
	namespace string,
	providerName string,
	providerVersion string,
	gpgKeyID string,
	sha256sumsPath string,
) error {
	c.log.Infow(
		"creating provider version",
		"version", providerVersion,
		"provider", providerName,
		"namespace", namespace,
	)

	providerdata := newCreateVersionReq()
	providerdata.Data = &providerData{
		Type: "registry-providers",
		Attributes: &providerAttributes{
			Version: providerVersion,
			KeyID:   gpgKeyID,
		},
	}

	payloadBytes, err := json.Marshal(providerdata)
	if err != nil {
		return err
	}
	databody := bytes.NewReader(payloadBytes)

	req, _ := NewTFCRequest(WithRequestBody(databody))
	req.ReqPath = fmt.Sprintf("api/v2/organizations/%s/registry-providers/private/%s/%s/versions", c.org, namespace, providerName)
	req.HTTPMethod = "POST"

	// "https://app.terraform.io/api/v2/organizations/hashicorp-qti/registry-providers/private/hashicorp-qti/enos-provider-dev/versions/"
	tfcAPIResp, err := c.DoRequest(ctx, req)
	if err != nil {
		return err
	}
	defer tfcAPIResp.Body.Close()

	if tfcAPIResp.StatusCode != http.StatusCreated {
		return newTFCAPIError("creating provider version", withErrTFCAPIResponse(tfcAPIResp))
	}

	body, err := io.ReadAll(tfcAPIResp.Body)
	if err != nil {
		return fmt.Errorf("reading provider version response: %w", err)
	}

	providerVersionBody := newCreateVersionRes()
	err = json.Unmarshal(body, &providerVersionBody)
	if err != nil {
		return fmt.Errorf("unmarshaling create provider version response: %w", err)
	}

	c.log.Infow(
		"created provider version",
		"provider version", providerVersionBody.Data.Attributes.Version,
		"provider sha sum uploaded?", providerVersionBody.Data.Attributes.SHAUploaded,
		"provider sha sign uploaded?", providerVersionBody.Data.Attributes.SHASigUploaded,
		"provider sha sum upload URL", providerVersionBody.Data.Links.ShasumsUpload,
		"provider sha sign upload URL", providerVersionBody.Data.Links.ShasumssigUpload,
	)

	// Upload Shasms file
	err = c.uploadFile(ctx, sha256sumsPath, providerVersionBody.Data.Links.ShasumsUpload)
	if err != nil {
		return fmt.Errorf("failed to upload shasums file for version created: %w", err)
	}

	// Upload Shasms signature file
	sha256sigPath := fmt.Sprintf("%s.sig", sha256sumsPath)
	err = c.uploadFile(ctx, sha256sigPath, providerVersionBody.Data.Links.ShasumssigUpload)
	if err != nil {
		return fmt.Errorf("failed to upload shasums sig file for version created: %w", err)
	}
	return err
}

// FindOrCreatePlatform - FindProviderPlatform - CreateProviderPlatforms
func (c *TFCClient) FindOrCreatePlatform(
	ctx context.Context,
	namespace string,
	providerName string,
	providerVersion string,
	release []*TFCRelease,
) error {
	platforms, err := c.FindProviderPlatform(ctx, namespace, providerName, providerVersion)
	if err != nil {
		return fmt.Errorf("error finding provider platform %w", err)
	}
	if len(platforms) == 0 {
		return c.CreateProviderPlatforms(ctx, namespace, providerName, providerVersion, release)
	}
	return nil
}

// FindProviderPlatform searches for platforms supported by a private provider version
func (c *TFCClient) FindProviderPlatform(
	ctx context.Context,
	namespace string,
	providerName string,
	providerVersion string,
) ([]*TFCProviderPlatform, error) {
	c.log.Infow(
		"searching provider platforms",
		"version", providerVersion,
		"provider name", providerName,
		"namespace", namespace,
	)

	req, _ := NewTFCRequest()
	req.ReqPath = fmt.Sprintf("api/v2/organizations/%s/registry-providers/private/%s/%s/versions/%s/platforms", c.org, namespace, providerName, providerVersion)
	req.HTTPMethod = "GET"

	//  https://app.terraform.io/api/v2/organizations/<ORG>/registry-providers/private/<NAMESPACE>/<NAME>/versions/<VERSION>/platforms"
	tfcAPIResp, err := c.DoRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	defer tfcAPIResp.Body.Close()

	if tfcAPIResp.StatusCode != http.StatusOK {
		return nil, newTFCAPIError("error searching provider platform", withErrTFCAPIResponse(tfcAPIResp))
	}

	body, err := io.ReadAll(tfcAPIResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading provider platforms response: %w", err)
	}

	providerPlatformBody := newProvidersPlatformData()
	err = json.Unmarshal(body, providerPlatformBody)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling provider platforms response: %w", err)
	}

	platforms := []*TFCProviderPlatform{}

	for _, data := range providerPlatformBody.Data {
		platform := &TFCProviderPlatform{
			Version:                providerVersion,
			PlatformID:             data.PlatformID,
			OS:                     data.Attributes.OS,
			Arch:                   data.Attributes.Arch,
			Filename:               data.Attributes.Filename,
			SHAsum:                 data.Attributes.SHAsum,
			PlatformBinaryUploaded: data.Attributes.BinaryUploaded,
		}
		if platform.PlatformBinaryUploaded {
			platform.PlatformBinaryURL = data.Links.ProviderBinaryDownload
		} else {
			platform.PlatformBinaryURL = data.Links.ProviderBinaryUpload
		}
		c.log.Infow(
			"found provider platform",
			"provider version", providerVersion,
			"provider platform id", platform.PlatformID,
			"provider platform os", platform.OS,
			"provider platform arch", platform.Arch,
			"provider platform filename", platform.Filename,
			"provider platform shasum", platform.SHAsum,
			"provider platform binary uploaded?", platform.PlatformBinaryUploaded,
			"provider platform URL", platform.PlatformBinaryURL,
		)
		platforms = append(platforms, platform)
	}
	return platforms, nil
}

// CreateProviderPlatforms creates platforms for a provider version
func (c *TFCClient) CreateProviderPlatforms(
	ctx context.Context,
	namespace string,
	providerName string,
	providerVersion string,
	releases []*TFCRelease,
) error {
	c.log.Infow(
		"creating provider platforms for",
		"version", providerVersion,
		"provider", providerName,
		"namespace", namespace,
	)

	for _, release := range releases {
		c.log.Infow(
			"creating platform",
			"os", release.Platform,
			"arch", release.Arch,
			"shasum", release.SHA256Sum,
			"file path", release.ZipFilePath,
		)

		req := newcreatePlatformReq()
		req.Data = &createPlatformReqData{
			Type: "registry-provider-platforms",
			Attributes: &createPlatformReqDataAttrs{
				OS:       release.Platform,
				Arch:     release.Arch,
				SHAsum:   release.SHA256Sum,
				Filename: filepath.Base(release.ZipFilePath),
			},
		}

		payloadBytes, err := json.Marshal(req)
		if err != nil {
			return err
		}
		databody := bytes.NewReader(payloadBytes)

		tfcreq, _ := NewTFCRequest(WithRequestBody(databody))
		tfcreq.ReqPath = fmt.Sprintf("api/v2/organizations/%s/registry-providers/private/%s/%s/versions/%s/platforms", c.org, namespace, providerName, providerVersion)
		tfcreq.HTTPMethod = "POST"

		//  "https://app.terraform.io/api/v2/organizations/hashicorp-qti/registry-providers/private/hashicorp-qti/enos-provider-dev/versions/0.1.20/platforms"
		tfcAPIResp, err := c.DoRequest(ctx, tfcreq)
		if err != nil {
			return err
		}
		defer tfcAPIResp.Body.Close()

		if tfcAPIResp.StatusCode != http.StatusCreated {
			return newTFCAPIError("error creating provider platform", withErrTFCAPIResponse(tfcAPIResp))
		}

		body, err := io.ReadAll(tfcAPIResp.Body)
		if err != nil {
			return fmt.Errorf("reading provider platforms response: %w", err)
		}

		providerPlatformBody := newCreatePlatformRes()
		err = json.Unmarshal(body, &providerPlatformBody)
		if err != nil {
			return fmt.Errorf("unmarshaling create provider platforms response: %w", err)
		}

		c.log.Infow(
			"created provider platform",
			"provider version", providerVersion,
			"provider platform id", providerPlatformBody.Data.PlatformID,
			"platform os", providerPlatformBody.Data.Attributes.OS,
			"platform arch", providerPlatformBody.Data.Attributes.Arch,
			"platform filepath", release.ZipFilePath,
			"platform shasum", providerPlatformBody.Data.Attributes.SHAsum,
			"platform binary uploaded?", providerPlatformBody.Data.Attributes.BinaryUploaded,
			"platform binary url", providerPlatformBody.Data.Links.ProviderBinaryUpload,
		)

		// Upload binary for platform created
		if !providerPlatformBody.Data.Attributes.BinaryUploaded {
			err := c.uploadFile(ctx, release.ZipFilePath, providerPlatformBody.Data.Links.ProviderBinaryUpload)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// uploadFile uploads a file to the URL
func (c *TFCClient) uploadFile(ctx context.Context, path string, uploadurl string) error {
	c.log.Infow(
		"uploading file",
		"file path", path,
		"upload URL", uploadurl,
	)
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	f, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer f.Close()

	u, err := url.Parse(uploadurl)
	if err != nil {
		return err
	}
	postURL := u.String()

	req, err := http.NewRequestWithContext(ctx, "PUT", postURL, f)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return newTFCAPIError("uploading file", withErrTFCAPIResponse(resp))
	}
	return nil
}
