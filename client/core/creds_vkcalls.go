package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/google/uuid"
)

const (
	vkConnectClientID     = "8093730"
	vkCallsAPIHost        = "api.vk.me"
	vkCallsAnonAPIVersion = "5.276"
)

var vkCallsProfile = Profile{
	UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
	SecChUa:         `"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`,
	SecChUaMobile:   "?0",
	SecChUaPlatform: `"Windows"`,
}

type vkCallsFailureKind string

const (
	vkCallsFailureSkipped vkCallsFailureKind = "skipped"
	vkCallsFailureSetup   vkCallsFailureKind = "setup"
	vkCallsFailureNetwork vkCallsFailureKind = "network"
	vkCallsFailureDecode  vkCallsFailureKind = "decode"
	vkCallsFailureVKAPI   vkCallsFailureKind = "vk_api"
	vkCallsFailureCaptcha vkCallsFailureKind = "captcha"
	vkCallsFailureOKCDN   vkCallsFailureKind = "okcdn_api"
	vkCallsFailureParse   vkCallsFailureKind = "parse"
)

type vkCallsFailure struct {
	Step string
	Kind vkCallsFailureKind
	Err  error
}

func (e *vkCallsFailure) Error() string {
	if e == nil {
		return "vkcalls failure"
	}
	if e.Err == nil {
		return fmt.Sprintf("step=%s kind=%s", e.Step, e.Kind)
	}
	return fmt.Sprintf("step=%s kind=%s: %v", e.Step, e.Kind, e.Err)
}

func (e *vkCallsFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newVKCallsFailure(step string, kind vkCallsFailureKind, err error) error {
	if err == nil {
		err = fmt.Errorf("unknown error")
	}
	return &vkCallsFailure{Step: step, Kind: kind, Err: err}
}

func describeVKCallsFailure(err error) string {
	if err == nil {
		return ""
	}
	var failure *vkCallsFailure
	if errors.As(err, &failure) {
		return failure.Error()
	}
	return err.Error()
}

func vkCallsAPIErrorKind(err error) vkCallsFailureKind {
	var captchaErr *VkCaptchaError
	if errors.As(err, &captchaErr) {
		return vkCallsFailureCaptcha
	}
	return vkCallsFailureVKAPI
}

type vkCallsVKAPIError struct {
	Code    int
	Message string
}

func (e *vkCallsVKAPIError) Error() string {
	if e == nil {
		return "VK API error"
	}
	if e.Message == "" {
		return fmt.Sprintf("error_code=%d", e.Code)
	}
	return fmt.Sprintf("error_code=%d %s", e.Code, e.Message)
}

type vkCallsOKAPIError struct {
	Code    int
	Message string
}

func (e *vkCallsOKAPIError) Error() string {
	if e == nil {
		return "OK CDN API error"
	}
	if e.Message == "" {
		return fmt.Sprintf("error_code=%d", e.Code)
	}
	return fmt.Sprintf("error_code=%d %s", e.Code, e.Message)
}

func getVKCredsViaVKCallsPath(ctx context.Context, link string, streamID int) (string, string, []string, error) {
	deviceID := uuid.New().String()
	name := generateName()
	profile := vkCallsProfile
	linkURL := url.QueryEscape("https://vk.com/call/join/" + link)
	nameEnc := url.QueryEscape(name)

	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithTimeoutSeconds(20),
		tlsclient.WithClientProfile(profiles.Chrome_146),
		tlsclient.WithCookieJar(tlsclient.NewCookieJar()),
	)
	if err != nil {
		return "", "", nil, newVKCallsFailure("setup", vkCallsFailureSetup, fmt.Errorf("create tls client: %w", err))
	}

	log.Printf("[STREAM %d] [VKCalls] Identity - Name: %s | device_id=%s | TLS=Chrome_146", streamID, name, deviceID)

	doRequest := func(step string, reqURL string) (map[string]interface{}, error) {
		req, err := fhttp.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(nil))
		if err != nil {
			return nil, newVKCallsFailure(step, vkCallsFailureSetup, fmt.Errorf("create request: %w", err))
		}
		req.Header.Set("User-Agent", profile.UserAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
		req.Header.Set("Accept-Language", "en-GB,en;q=0.9")

		httpResp, err := client.Do(req)
		if err != nil {
			return nil, newVKCallsFailure(step, vkCallsFailureNetwork, fmt.Errorf("request failed: %w", err))
		}
		defer httpResp.Body.Close()

		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, newVKCallsFailure(step, vkCallsFailureNetwork, fmt.Errorf("read response: %w", err))
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, newVKCallsFailure(step, vkCallsFailureDecode, fmt.Errorf("unmarshal JSON: %w, body: %s", err, truncateVKCallsLog(string(body), 200)))
		}
		return resp, nil
	}

	// Step 1: auth.getAnonymToken
	step1 := "step1 auth.getAnonymToken"
	step1URL := fmt.Sprintf(
		"https://%s/method/auth.getAnonymToken?v=%s&client_id=%s&link=%s&device_id=%s&anonymName=%s&lang=en",
		vkCallsAPIHost, vkCallsAnonAPIVersion, vkConnectClientID,
		linkURL, deviceID, nameEnc,
	)
	resp1, err := doRequest(step1, step1URL)
	if err != nil {
		return "", "", nil, err
	}
	anonymToken, err := extractVKCallsStr(resp1, "response", "token")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step1, vkCallsFailureParse, fmt.Errorf("parse token: %w (resp: %s)", err, truncateVKCallsResp(resp1)))
	}
	anonymTokenEnc := url.QueryEscape(anonymToken)
	log.Printf("[STREAM %d] [VKCalls] step1 OK, anonymous_token (%d chars)", streamID, len(anonymToken))

	// Step 2: messages.getCallPreview
	step2 := "step2 messages.getCallPreview"
	step2URL := fmt.Sprintf(
		"https://%s/method/messages.getCallPreview?v=%s&anonymous_token=%s&device_id=%s&extended=1&fields=first_name,last_name,photo_200&lang=en&link=%s",
		vkCallsAPIHost, vkCallsAnonAPIVersion, anonymTokenEnc, deviceID, linkURL,
	)
	resp2, err := doRequest(step2, step2URL)
	if err != nil {
		return "", "", nil, err
	}
	if apiErr := vkCallsCheckAPIError(resp2); apiErr != nil {
		if captchaErr, ok := apiErr.(*VkCaptchaError); ok {
			log.Printf("[STREAM %d] [VKCalls] step2 captcha gate (sid=%q)", streamID, captchaErr.CaptchaSid)
		}
		return "", "", nil, newVKCallsFailure(step2, vkCallsAPIErrorKind(apiErr), apiErr)
	}
	userIDFloat, err := extractVKCallsFloat(resp2, "response", "user_id")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step2, vkCallsFailureParse, fmt.Errorf("parse user_id: %w (resp: %s)", err, truncateVKCallsResp(resp2)))
	}
	userIDStr := fmt.Sprintf("%.0f", userIDFloat)
	secret, err := extractVKCallsStr(resp2, "response", "secret")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step2, vkCallsFailureParse, fmt.Errorf("parse secret: %w", err))
	}
	log.Printf("[STREAM %d] [VKCalls] step2 OK, user_id=%s", streamID, userIDStr)

	// Step 3: messages.getAnonymCallToken
	step3 := "step3 messages.getAnonymCallToken"
	step3URL := fmt.Sprintf(
		"https://%s/method/messages.getAnonymCallToken?v=%s&anonymous_token=%s&device_id=%s&link=%s&name=%s&user_id=%s&secret=%s&lang=en",
		vkCallsAPIHost, vkCallsAnonAPIVersion, anonymTokenEnc, deviceID, linkURL,
		nameEnc, userIDStr, url.QueryEscape(secret),
	)
	resp3, err := doRequest(step3, step3URL)
	if err != nil {
		return "", "", nil, err
	}
	if apiErr := vkCallsCheckAPIError(resp3); apiErr != nil {
		if captchaErr, ok := apiErr.(*VkCaptchaError); ok {
			log.Printf("[STREAM %d] [VKCalls] step3 captcha gate (sid=%q)", streamID, captchaErr.CaptchaSid)
		}
		return "", "", nil, newVKCallsFailure(step3, vkCallsAPIErrorKind(apiErr), apiErr)
	}
	okAnonymToken, err := extractVKCallsStr(resp3, "response", "token")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step3, vkCallsFailureParse, fmt.Errorf("parse token: %w (resp: %s)", err, truncateVKCallsResp(resp3)))
	}
	log.Printf("[STREAM %d] [VKCalls] step3 OK, OK anonymToken (%d chars)", streamID, len(okAnonymToken))

	// Step 4: OK CDN auth.anonymLogin
	okDeviceID := uuid.New().String()
	step4 := "step4 auth.anonymLogin"
	step4URL := "https://calls.okcdn.ru/fb.do?session_data=" +
		url.QueryEscape(fmt.Sprintf(
			`{"version":2,"device_id":"%s","client_version":"1.0.1"}`, okDeviceID,
		)) +
		"&method=auth.anonymLogin&format=JSON&application_key=CGMMEJLGDIHBABABA"
	resp4, err := doRequest(step4, step4URL)
	if err != nil {
		return "", "", nil, err
	}
	sessionKey, err := extractVKCallsStr(resp4, "session_key")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step4, vkCallsFailureParse, fmt.Errorf("parse session_key: %w (resp: %s)", err, truncateVKCallsResp(resp4)))
	}
	log.Printf("[STREAM %d] [VKCalls] step4 OK, session_key (%d chars)", streamID, len(sessionKey))

	// Step 5: vchat.joinConversationByLink
	step5 := "step5 vchat.joinConversationByLink"
	step5URL := fmt.Sprintf(
		"https://calls.okcdn.ru/fb.do?joinLink=%s&isVideo=false&protocolVersion=5&anonymToken=%s&method=vchat.joinConversationByLink&format=JSON&application_key=CGMMEJLGDIHBABABA&session_key=%s",
		link, okAnonymToken, sessionKey,
	)
	resp5, err := doRequest(step5, step5URL)
	if err != nil {
		return "", "", nil, err
	}
	if okErr := vkCallsCheckOKError(resp5); okErr != nil {
		return "", "", nil, newVKCallsFailure(step5, vkCallsFailureOKCDN, fmt.Errorf("%w (resp: %s)", okErr, truncateVKCallsResp(resp5)))
	}

	user, err := extractVKCallsStr(resp5, "turn_server", "username")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step5, vkCallsFailureParse, fmt.Errorf("parse username: %w (resp: %s)", err, truncateVKCallsResp(resp5)))
	}
	pass, err := extractVKCallsStr(resp5, "turn_server", "credential")
	if err != nil {
		return "", "", nil, newVKCallsFailure(step5, vkCallsFailureParse, fmt.Errorf("parse credential: %w (resp: %s)", err, truncateVKCallsResp(resp5)))
	}
	addrs := parseVKCallsTURNAddresses(resp5)
	if len(addrs) == 0 {
		return "", "", nil, newVKCallsFailure(step5, vkCallsFailureParse, fmt.Errorf("turn_server.urls empty"))
	}

	log.Printf("[STREAM %d] [VKCalls] SUCCESS, TURN urls=%d", streamID, len(addrs))
	return user, pass, addrs, nil
}

func extractVKCallsStr(resp map[string]interface{}, keys ...string) (string, error) {
	var cur interface{} = resp
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("expected map at key %q, got %T", k, cur)
		}
		cur = m[k]
	}
	s, ok := cur.(string)
	if !ok {
		return "", fmt.Errorf("expected string at end of path, got %T", cur)
	}
	return s, nil
}

func extractVKCallsFloat(resp map[string]interface{}, keys ...string) (float64, error) {
	var cur interface{} = resp
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("expected map at key %q, got %T", k, cur)
		}
		cur = m[k]
	}
	f, ok := cur.(float64)
	if !ok {
		return 0, fmt.Errorf("expected float64 at end of path, got %T", cur)
	}
	return f, nil
}

func parseVKCallsTURNAddresses(resp map[string]interface{}) []string {
	turnServer, ok := resp["turn_server"].(map[string]interface{})
	if !ok {
		return nil
	}
	urls, ok := turnServer["urls"].([]interface{})
	if !ok {
		return nil
	}
	var addrs []string
	for _, u := range urls {
		s, ok := u.(string)
		if !ok {
			continue
		}
		clean := strings.Split(s, "?")[0]
		addr := strings.TrimPrefix(strings.TrimPrefix(clean, "turn:"), "turns:")
		log.Printf("[VKCalls] turn url: %s", addr)
		addrs = append(addrs, addr)
	}
	return addrs
}

func vkCallsCheckAPIError(resp map[string]interface{}) error {
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		return nil
	}
	code, _ := errObj["error_code"].(float64)
	msg, _ := errObj["error_msg"].(string)
	if code == 0 && msg == "" {
		return nil
	}
	if int(code) == 14 {
		return parseVkCaptchaError(errObj)
	}
	return &vkCallsVKAPIError{Code: int(code), Message: msg}
}

func vkCallsCheckOKError(resp map[string]interface{}) error {
	code, ok := resp["error_code"].(float64)
	if !ok || code == 0 {
		return nil
	}
	msg, _ := resp["error_msg"].(string)
	return &vkCallsOKAPIError{Code: int(code), Message: msg}
}

func truncateVKCallsLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func truncateVKCallsResp(resp map[string]interface{}) string {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf("(unmarshallable: %v)", err)
	}
	return truncateVKCallsLog(string(b), 300)
}
