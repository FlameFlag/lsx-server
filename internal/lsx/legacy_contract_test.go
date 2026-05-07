package lsx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type legacyContract struct {
	Protocol map[string]protocolSpec `yaml:"protocol"`
	Setup    []contractCase          `yaml:"setup"`
	Cases    []contractCase          `yaml:"examples"`
}

type protocolSpec struct {
	Method              string            `yaml:"method"`
	Host                string            `yaml:"host"`
	RequestHeaders      map[string]string `yaml:"request_headers"`
	Routes              []string          `yaml:"routes"`
	RequiredQuery       []string          `yaml:"required_query"`
	ResponseContains    string            `yaml:"response_contains"`
	ResponseContentType string            `yaml:"response_content_type"`
}

type contractCase struct {
	Name           string           `yaml:"name"`
	Contract       string           `yaml:"contract"`
	StrictChecksum bool             `yaml:"strict_checksum"`
	Request        contractRequest  `yaml:"request"`
	Response       contractResponse `yaml:"response"`
}

type contractRequest struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Query   map[string]string `yaml:"query"`
	Headers map[string]string `yaml:"headers"`
}

type contractResponse struct {
	Status       int      `yaml:"status"`
	Body         string   `yaml:"body"`
	ContentType  string   `yaml:"content_type"`
	BodyNonempty bool     `yaml:"body_nonempty"`
	Contains     []string `yaml:"contains"`
	NotContains  []string `yaml:"not_contains"`
}

func TestLegacyContract(t *testing.T) {
	var contract legacyContract
	readYAMLFixture(t, "testdata/legacy_contract.yaml", &contract)
	assertProtocolSpecs(t, contract.Protocol)

	srv := newContractServer(t, false)
	for _, setup := range contract.Setup {
		assertContractCase(t, srv, contract.Protocol, setup)
	}

	for _, tc := range contract.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			caseSrv := srv
			if tc.StrictChecksum {
				caseSrv = newContractServer(t, true)
				for _, setup := range contract.Setup {
					assertContractCase(t, caseSrv, contract.Protocol, setup)
				}
			}
			assertContractCase(t, caseSrv, contract.Protocol, tc)
		})
	}
}

func TestOpenAPIYAMLRouteUsesConfiguredContract(t *testing.T) {
	srv, err := NewServer(Config{
		DBPath:      filepath.Join(t.TempDir(), "lsx.sqlite3"),
		OpenAPIYAML: []byte("openapi: 3.1.0\ninfo:\n  title: embedded test\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	rr := getRoute(t, srv, "/openapi.yaml")
	if rr.Code != http.StatusOK {
		t.Fatalf("/openapi.yaml status = %d, want 200", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "title: embedded test") {
		t.Fatalf("/openapi.yaml body = %q, want configured contract", body)
	}
}

func newContractServer(t *testing.T, strictChecksum bool) *Server {
	t.Helper()
	srv, err := NewServer(Config{
		DBPath:         filepath.Join(t.TempDir(), "lsx.sqlite3"),
		StrictChecksum: strictChecksum,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func assertContractCase(t *testing.T, srv *Server, specs map[string]protocolSpec, tc contractCase) {
	t.Helper()
	target := tc.Request.target()
	if target == "" {
		t.Fatal("contract case path is empty")
	}
	spec := assertCaseMatchesProtocol(t, specs, tc, target)
	method := tc.requestMethod(spec)
	req := httptest.NewRequest(method, target, nil)
	if spec.Host != "" {
		req.Host = spec.Host
	}
	for key, value := range spec.RequestHeaders {
		req.Header.Set(key, value)
	}
	for key, value := range tc.Request.Headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)
	result := rr.Result()

	if result.StatusCode != tc.Response.Status {
		t.Fatalf("%s status = %d, want %d; body=%q", target, result.StatusCode, tc.Response.Status, rr.Body.String())
	}
	if tc.Response.ContentType != "" {
		if got := result.Header.Get("Content-Type"); !strings.HasPrefix(got, tc.Response.ContentType) {
			t.Fatalf("%s content-type = %q, want prefix %q", target, got, tc.Response.ContentType)
		}
	}
	body := rr.Body.String()
	if tc.Response.Body != "" && body != tc.Response.Body {
		t.Fatalf("%s body = %q, want %q", target, body, tc.Response.Body)
	}
	if spec.ResponseContains != "" && !strings.Contains(body, spec.ResponseContains) {
		t.Fatalf("%s body does not contain protocol token %q", target, spec.ResponseContains)
	}
	if tc.Response.BodyNonempty && rr.Body.Len() == 0 {
		t.Fatalf("%s body is empty", target)
	}
	for _, want := range tc.Response.Contains {
		if !strings.Contains(body, want) {
			t.Fatalf("%s body does not contain %q", target, want)
		}
	}
	for _, unwanted := range tc.Response.NotContains {
		if strings.Contains(body, unwanted) {
			t.Fatalf("%s body unexpectedly contains %q", target, unwanted)
		}
	}
}

func assertProtocolSpecs(t *testing.T, specs map[string]protocolSpec) {
	t.Helper()
	for name, spec := range specs {
		if spec.Method != "" && spec.Method != http.MethodGet {
			t.Fatalf("protocol %q method = %q, only GET is currently supported by this test harness", name, spec.Method)
		}
		if len(spec.Routes) == 0 {
			t.Fatalf("protocol %q has no routes", name)
		}
		if spec.ResponseContains == "" && spec.ResponseContentType == "" {
			t.Fatalf("protocol %q has no response constraint", name)
		}
	}
}

func assertCaseMatchesProtocol(t *testing.T, specs map[string]protocolSpec, tc contractCase, target string) protocolSpec {
	t.Helper()
	if tc.Contract == "" {
		return protocolSpec{}
	}
	spec, ok := specs[tc.Contract]
	if !ok {
		t.Fatalf("%s references unknown protocol contract %q", target, tc.Contract)
	}
	u, err := url.Parse(target)
	if err != nil {
		t.Fatalf("parse %s: %v", target, err)
	}
	if !containsString(spec.Routes, u.Path) {
		t.Fatalf("%s path %q is not one of protocol %q routes %v", target, u.Path, tc.Contract, spec.Routes)
	}
	for _, key := range spec.RequiredQuery {
		if _, ok := u.Query()[key]; !ok {
			t.Fatalf("%s is missing required %s query field %q", target, tc.Contract, key)
		}
	}
	if spec.ResponseContentType != "" && tc.Response.ContentType == "" {
		t.Fatalf("%s uses protocol %q but does not assert content type %q", target, tc.Contract, spec.ResponseContentType)
	}
	if tc.Response.Status == 0 {
		t.Fatalf("%s has no expected response status", target)
	}
	if tc.Response.Body == "" && tc.Response.ContentType == "" && !tc.Response.BodyNonempty && len(tc.Response.Contains) == 0 && len(tc.Response.NotContains) == 0 {
		t.Fatalf("%s has no response body/header assertion", target)
	}
	return spec
}

func (tc contractCase) requestMethod(spec protocolSpec) string {
	if tc.Request.Method != "" {
		return tc.Request.Method
	}
	if spec.Method != "" {
		return spec.Method
	}
	return http.MethodGet
}

func (req contractRequest) target() string {
	if req.Path == "" {
		return ""
	}
	if len(req.Query) == 0 {
		return req.Path
	}
	values := make(url.Values, len(req.Query))
	for key, value := range req.Query {
		values.Set(key, value)
	}
	return req.Path + "?" + values.Encode()
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func readYAMLFixture(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
