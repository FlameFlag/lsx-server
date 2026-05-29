package lsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lt2_reverse/lsx_server_go/internal/lsx/keygen"
)

// isArmadilloRequest detects Armadillo DRM traffic by User-Agent, Content-Type,
// or SOAPAction header. Only POST requests can be Armadillo SOAP calls.
func isArmadilloRequest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	ua := strings.ToLower(r.UserAgent())
	if strings.Contains(ua, "armadillo") {
		return true
	}
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(ct, "xml") {
		return true
	}
	if r.Header.Get("SOAPAction") != "" {
		return true
	}
	return false
}

// handleActivate serves two roles:
//   - Browser GET -> renders the activation setup guide page (Svelte)
//   - SOAP POST   -> responds to Armadillo/SoftwarePassport DRM activation

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	if isArmadilloRequest(r) {
		s.handleDRMSOAP(w, r)
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		s.handleActivatePage(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDRMSOAP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(io.LimitReader(r.Body, 65536))
	}
	bodyStr := string(body)

	action := classifySOAPAction(r.Header.Get("SOAPAction"), bodyStr)
	s.emit(r, "drm_activation", http.StatusOK, "action="+action)

	response := buildSOAPResponse(action, bodyStr)
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(response)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response)
}

func classifySOAPAction(soapAction, body string) string {
	lower := strings.ToLower(soapAction + " " + body)
	switch {
	case strings.Contains(lower, "activatelicense"):
		return "activateLicense"
	case strings.Contains(lower, "validatelicense"):
		return "validateLicense"
	case strings.Contains(lower, "reissuekey"):
		return "reissueKey"
	case strings.Contains(lower, "generatekeyfornotrial"):
		return "generateKeyForNoTrial"
	case strings.Contains(lower, "generatekey"):
		return "generateKey"
	default:
		return "unknown"
	}
}

func buildSOAPResponse(action, requestBody string) []byte {
	entitlementID := extractSOAPTag(requestBody, "entitlementID")
	userName := extractSOAPTag(requestBody, "userName")
	key := ""

	if entitlementID == "" {
		entitlementID = fmt.Sprintf("LT2-%08X", uint32(time.Now().Unix()))
	}
	if userName == "" {
		userName = "Licensed User"
	}
	if soapActionReturnsKey(action) {
		pair, err := keygen.Generate(userName)
		if err == nil {
			userName = pair.RegistrationName
			key = pair.ActivationKey
		}
	}

	actionTag := action + "Response"
	ns := "http://webservice.digitalright.digitalriver.com/DigitalRight"
	keyElement := ""
	if key != "" {
		keyElement = fmt.Sprintf("\n      <key xsi:type=\"xsd:string\">%s</key>", xmlEscapeText(key))
	}

	return fmt.Appendf(nil,
		`<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope
  xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/"
  xmlns:xsd="http://www.w3.org/2001/XMLSchema"
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <SOAP-ENV:Body>
	    <ns1:%s xmlns:ns1="%s">
	      <result xsi:type="xsd:int">0</result>
	      <entitlementID xsi:type="xsd:string">%s</entitlementID>
	      <userName xsi:type="xsd:string">%s</userName>%s
	    </ns1:%s>
	  </SOAP-ENV:Body>
	</SOAP-ENV:Envelope>
`, actionTag, ns, xmlEscapeText(entitlementID), xmlEscapeText(userName), keyElement, actionTag)
}

func soapActionReturnsKey(action string) bool {
	switch action {
	case "generateKey", "reissueKey", "generateKeyForNoTrial", "activateLicense":
		return true
	default:
		return false
	}
}

func extractSOAPTag(xmlStr, tag string) string {
	dec := xml.NewDecoder(strings.NewReader(xmlStr))
	for {
		token, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != tag {
			continue
		}
		var content strings.Builder
		depth := 1
		for depth > 0 {
			token, err = dec.Token()
			if err != nil {
				return strings.TrimSpace(content.String())
			}
			switch t := token.(type) {
			case xml.CharData:
				if depth == 1 {
					content.Write([]byte(t))
				}
			case xml.StartElement:
				depth++
			case xml.EndElement:
				depth--
			}
		}
		return strings.TrimSpace(content.String())
	}

	if v := extractTagExact(xmlStr, tag); v != "" {
		return v
	}
	for _, prefix := range []string{"ns1:", "m:", "ns:", "soap:"} {
		if v := extractTagExact(xmlStr, prefix+tag); v != "" {
			return v
		}
	}
	return ""
}

func extractTagExact(xmlStr, tag string) string {
	openTag := "<" + tag
	closeTag := "</" + tag + ">"

	start := strings.Index(xmlStr, openTag)
	if start == -1 {
		return ""
	}
	tagEnd := strings.Index(xmlStr[start:], ">")
	if tagEnd == -1 {
		return ""
	}
	contentStart := start + tagEnd + 1
	if xmlStr[contentStart-2] == '/' {
		return ""
	}
	end := strings.Index(xmlStr[contentStart:], closeTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(xmlStr[contentStart : contentStart+end])
}

func xmlEscapeText(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
