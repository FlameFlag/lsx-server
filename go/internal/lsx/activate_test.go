package lsx

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestDRMSOAPActionsMatchRecoveredArmadilloOperations(t *testing.T) {
	srv, err := NewServer(Config{DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	actions := []string{
		"activateLicense",
		"validateLicense",
		"generateKey",
		"reissueKey",
		"generateKeyForNoTrial",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			body := `<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/" xmlns:m="http://digitalriver.com/DigitalRight">
  <SOAP-ENV:Body>
    <m:` + action + `>
      <m:entitlementID>ENT-42</m:entitlementID>
      <m:userName>Ada &amp; Bob</m:userName>
    </m:` + action + `>
  </SOAP-ENV:Body>
</SOAP-ENV:Envelope>`

			req := httptest.NewRequest(http.MethodPost, "/activate", strings.NewReader(body))
			req.Header.Set("Content-Type", "text/xml; charset=utf-8")
			req.Header.Set("User-Agent", "ArmadilloDRM/1.0")
			req.Header.Set("SOAPAction", `"http://digitalriver.com/DigitalRight/`+action+`"`)
			rr := httptest.NewRecorder()
			srv.Routes().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%q", rr.Code, rr.Body.String())
			}
			if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/xml") {
				t.Fatalf("content-type = %q, want text/xml", got)
			}
			resp := rr.Body.String()
			for _, want := range []string{
				"<ns1:" + action + "Response",
				`xmlns:ns1="http://webservice.digitalright.digitalriver.com/DigitalRight"`,
				"<result xsi:type=\"xsd:int\">0</result>",
				"<entitlementID xsi:type=\"xsd:string\">ENT-42</entitlementID>",
				"<userName xsi:type=\"xsd:string\">Ada &amp; Bob</userName>",
			} {
				if !strings.Contains(resp, want) {
					t.Fatalf("response missing %q:\n%s", want, resp)
				}
			}
			if action == "validateLicense" {
				if strings.Contains(resp, "<key ") {
					t.Fatalf("validateLicense response should not include key material:\n%s", resp)
				}
			} else if !strings.Contains(resp, "<key xsi:type=\"xsd:string\">") {
				t.Fatalf("key-returning response missing key field:\n%s", resp)
			}
			assertWellFormedXML(t, resp)
		})
	}
}

func TestDRMSOAPCanArriveOnAnyActivationPath(t *testing.T) {
	srv, err := NewServer(Config{DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	req := httptest.NewRequest(http.MethodPost, "/DigitalRight/Service.asmx", strings.NewReader(`<validateLicense/>`))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"http://digitalriver.com/DigitalRight/validateLicense"`)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); !strings.Contains(body, "<ns1:validateLicenseResponse") {
		t.Fatalf("body does not contain validateLicense response:\n%s", body)
	}
}

func TestUnknownSOAPActionStillReturnsWellFormedSuccess(t *testing.T) {
	srv, err := NewServer(Config{DBPath: filepath.Join(t.TempDir(), "lsx.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	req := httptest.NewRequest(http.MethodPost, "/activate", strings.NewReader(`<notARecoveredAction/>`))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"http://digitalriver.com/DigitalRight/notARecoveredAction"`)
	rr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rr.Code, rr.Body.String())
	}
	resp := rr.Body.String()
	if !strings.Contains(resp, "<ns1:unknownResponse") {
		t.Fatalf("body does not contain unknown response:\n%s", resp)
	}
	assertWellFormedXML(t, resp)
}

func assertWellFormedXML(t *testing.T, data string) {
	t.Helper()
	var v any
	if err := xml.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("response is not well-formed XML: %v\n%s", err, data)
	}
}
