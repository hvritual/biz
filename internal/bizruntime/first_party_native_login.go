package bizruntime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

type firstPartyLoginPage struct {
	RequestID        string
	CSRF             string
	Identifier       string
	Message          string
	PrivacyPolicyURL string
	TermsURL         string
	OTPRequestNonce  string
	OTPChallengeID   string
	OTPSelected      bool
	OTPRequested     bool
	OTPEnabled       bool
	CodeDigits       int
}

func (idp *runtimeFirstPartyIdP) setVerification(service *accessapp.VerificationService) {
	if idp == nil {
		return
	}
	idp.mu.Lock()
	idp.verification = service
	idp.mu.Unlock()
}

func (idp *runtimeFirstPartyIdP) currentVerification() *accessapp.VerificationService {
	if idp == nil {
		return nil
	}
	idp.mu.RLock()
	defer idp.mu.RUnlock()
	return idp.verification
}

func (idp *runtimeFirstPartyIdP) renderLoginState(writer http.ResponseWriter, status int, page firstPartyLoginPage) {
	page.PrivacyPolicyURL = idp.config.PrivacyConsent.PrivacyPolicyURL
	page.TermsURL = idp.config.PrivacyConsent.TermsURL
	page.OTPEnabled = idp.currentVerification() != nil
	if page.CodeDigits == 0 {
		page.CodeDigits = idp.config.OTPCodeDigits
	}
	if page.CodeDigits == 0 {
		page.CodeDigits = 6
	}
	if page.OTPRequestNonce == "" {
		nonce, err := randomURLSecret(18)
		if err != nil {
			http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
			return
		}
		page.OTPRequestNonce = nonce
	}
	scriptNonce, err := randomURLSecret(18)
	if err != nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	idp.setLoginSecurityHeaders(writer, scriptNonce)
	writer.WriteHeader(status)
	_ = firstPartyLoginTemplate.Execute(writer, struct {
		firstPartyLoginPage
		ScriptNonce string
	}{firstPartyLoginPage: page, ScriptNonce: scriptNonce})
}

func (idp *runtimeFirstPartyIdP) handleOTPRequest(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	verification := idp.currentVerification()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid login request", http.StatusBadRequest)
		return
	}
	requestID := strings.TrimSpace(request.Form.Get("request_id"))
	csrf := strings.TrimSpace(request.Form.Get("csrf_token"))
	identifier := strings.TrimSpace(request.Form.Get("identifier"))
	sendNonce := strings.TrimSpace(request.Form.Get("otp_request_nonce"))
	cookie, err := request.Cookie(idp.authCookieName())
	if err != nil || requestID == "" || csrf == "" || sendNonce == "" || strings.TrimSpace(cookie.Value) == "" {
		http.Error(writer, "invalid login request", http.StatusUnauthorized)
		return
	}
	if _, err := store.LoadFirstPartyAuthorizationRequest(request.Context(), requestID, cookie.Value, csrf); err != nil {
		http.Error(writer, "invalid login request", http.StatusUnauthorized)
		return
	}
	if verification == nil {
		idp.renderLoginState(writer, http.StatusServiceUnavailable, firstPartyLoginPage{
			RequestID: requestID, CSRF: csrf, Identifier: identifier, OTPSelected: true,
			Message: "验证码登录当前未配置可用发送渠道。",
		})
		return
	}

	resolved, resolveErr := store.ResolveLoginIdentifier(request.Context(), identifier)
	if resolveErr != nil {
		fake, _ := randomURLSecret(24)
		idp.renderLoginState(writer, http.StatusOK, firstPartyLoginPage{
			RequestID: requestID, CSRF: csrf, Identifier: identifier, OTPSelected: true, OTPRequested: true,
			OTPChallengeID: "vch-fake-" + fake,
			Message: "验证码请求已受理；若账号可用且渠道正常，将发送验证码。",
		})
		return
	}

	challenge, _, sendErr := verification.SendVerificationCode(request.Context(), accessdomain.VerificationChallengeRequest{
		BusinessEventID: "idp-login-otp/" + requestID + "/" + accesspersistence.TokenHash(sendNonce),
		FlowID:          requestID,
		Purpose:         accessdomain.VerificationPurposeLogin,
		UserID:          resolved.Identity.UserID,
		Channel:         resolved.OTPChannel,
		Destination:     resolved.OTPDestination,
	})
	if challenge.ChallengeID == "" {
		fake, _ := randomURLSecret(24)
		challenge.ChallengeID = "vch-fake-" + fake
	}
	message := "验证码请求已受理；若账号可用且渠道正常，将发送验证码。"
	if sendErr != nil && errors.Is(sendErr, accessdomain.ErrVerificationRateLimited) {
		message = "验证码请求过于频繁，请稍后再试。"
	}
	idp.renderLoginState(writer, http.StatusOK, firstPartyLoginPage{
		RequestID: requestID, CSRF: csrf, Identifier: identifier, OTPSelected: true, OTPRequested: true,
		OTPChallengeID: challenge.ChallengeID, Message: message,
	})
}

func (idp *runtimeFirstPartyIdP) handleOTPVerify(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	verification := idp.currentVerification()
	if store == nil || verification == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid login request", http.StatusBadRequest)
		return
	}
	requestID := strings.TrimSpace(request.Form.Get("request_id"))
	csrf := strings.TrimSpace(request.Form.Get("csrf_token"))
	identifier := strings.TrimSpace(request.Form.Get("identifier"))
	challengeID := strings.TrimSpace(request.Form.Get("challenge_id"))
	code := strings.TrimSpace(request.Form.Get("otp_code"))
	remember := request.Form.Get("remember_identifier") == "true"
	cookie, err := request.Cookie(idp.authCookieName())
	if err != nil || requestID == "" || csrf == "" || challengeID == "" || code == "" || strings.TrimSpace(cookie.Value) == "" {
		http.Error(writer, "invalid login request", http.StatusUnauthorized)
		return
	}
	if _, err := store.LoadFirstPartyAuthorizationRequest(request.Context(), requestID, cookie.Value, csrf); err != nil {
		http.Error(writer, "invalid login request", http.StatusUnauthorized)
		return
	}
	resolved, err := store.ResolveLoginIdentifier(request.Context(), identifier)
	if err != nil {
		idp.renderLoginState(writer, http.StatusUnauthorized, firstPartyLoginPage{
			RequestID: requestID, CSRF: csrf, Identifier: identifier, OTPSelected: true, OTPRequested: true,
			OTPChallengeID: challengeID, Message: "账号或验证码错误。",
		})
		return
	}

	authorization, err := verification.VerifyCode(request.Context(), accessdomain.VerifyChallengeRequest{
		ChallengeID: challengeID, FlowID: requestID, Purpose: accessdomain.VerificationPurposeLogin,
		UserID: resolved.Identity.UserID, Channel: resolved.OTPChannel, Destination: resolved.OTPDestination, Code: code,
	})
	if err != nil {
		message := "账号或验证码错误。"
		switch {
		case errors.Is(err, accessdomain.ErrVerificationExpired):
			message = "验证码已过期，请重新获取。"
		case errors.Is(err, accessdomain.ErrVerificationConsumed):
			message = "验证码已使用，请重新获取。"
		case errors.Is(err, accessdomain.ErrVerificationRateLimited):
			message = "验证码错误次数过多，请重新获取。"
		}
		idp.renderLoginState(writer, http.StatusUnauthorized, firstPartyLoginPage{
			RequestID: requestID, CSRF: csrf, Identifier: identifier, OTPSelected: true, OTPRequested: true,
			OTPChallengeID: challengeID, Message: message,
		})
		return
	}
	if _, err := verification.ConsumeAuthorization(request.Context(), accessdomain.ConsumeAuthorizationRequest{
		Code: authorization.Code, FlowID: requestID, Purpose: accessdomain.VerificationPurposeLogin,
		UserID: resolved.Identity.UserID, Channel: resolved.OTPChannel, Destination: resolved.OTPDestination,
	}); err != nil {
		idp.renderLoginState(writer, http.StatusUnauthorized, firstPartyLoginPage{
			RequestID: requestID, CSRF: csrf, Identifier: identifier, OTPSelected: true,
			Message: "验证码授权已失效，请重新获取。",
		})
		return
	}
	auditID, err := store.RecordFirstPartyVerifiedLogin(request.Context(), identifier, resolved.Identity.UserID, request.RemoteAddr)
	if err != nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	idp.continueVerifiedLogin(writer, request, requestID, csrf, cookie.Value, identifier, remember, resolved.Identity, auditID)
}

func (idp *runtimeFirstPartyIdP) continueVerifiedLogin(
	writer http.ResponseWriter,
	request *http.Request,
	requestID, csrf, browserSecret, identifier string,
	remember bool,
	identity accesspersistence.LocalUserIdentity,
	loginAuditID uint64,
) {
	store := idp.currentStore()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	idp.updateRememberedIdentifierCookie(writer, identifier, remember)
	satisfied, err := store.PrivacyConsentSatisfies(
		request.Context(),
		identity.UserID,
		idp.config.PrivacyConsent.AgreementVersion,
		idp.config.PrivacyConsent.RequireCurrentVersion(),
	)
	if err != nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	if !satisfied {
		if err := store.BindFirstPartyAuthorizationIdentity(request.Context(), requestID, browserSecret, csrf, identity.UserID, loginAuditID); err != nil {
			http.Error(writer, "login transaction expired", http.StatusUnauthorized)
			return
		}
		location := "/idp/consent?request_id=" + url.QueryEscape(requestID)
		http.Redirect(writer, request, location, http.StatusSeeOther)
		return
	}
	code, authorization, err := store.IssueFirstPartyAuthorizationCode(request.Context(), requestID, browserSecret, csrf, identity.UserID, idp.config.CodeTTL)
	if err != nil {
		http.Error(writer, "login transaction expired", http.StatusUnauthorized)
		return
	}
	idp.finishAuthorization(writer, request, code, authorization)
}

func (idp *runtimeFirstPartyIdP) maybeNotifyLoginLock(
	ctx context.Context,
	resolved accesspersistence.LoginIdentifierResolution,
	requestID string,
	blockedUntil *time.Time,
) {
	verification := idp.currentVerification()
	if verification == nil || blockedUntil == nil || resolved.Identity.UserID == "" || resolved.OTPDestination == "" {
		return
	}
	eventID := fmt.Sprintf("idp-login-lock/%s/%d", resolved.Identity.UserID, blockedUntil.UTC().Unix())
	queued, err := verification.QueueSecurityNotification(ctx, accessdomain.SecurityNotificationRequest{
		BusinessEventID: eventID,
		Kind:            accessdomain.SecurityNotificationLoginLock,
		Purpose:         accessdomain.VerificationPurposeLogin,
		UserID:          resolved.Identity.UserID,
		FlowID:          requestID,
		Channel:         resolved.OTPChannel,
		Destination:     resolved.OTPDestination,
		ExpiresAt:       blockedUntil.UTC(),
	})
	if err != nil || queued.EventID == "" {
		return
	}
	_, _ = verification.DeliverSecurityNotification(ctx, queued.EventID)
}

func (idp *runtimeFirstPartyIdP) rememberedIdentifier(request *http.Request) string {
	cookie, err := request.Cookie(idp.loginHintCookieName())
	if err != nil {
		return ""
	}
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return ""
	}
	if _, normalized, err := accesspersistence.NormalizeLoginIdentifier(value); err == nil {
		return normalized
	}
	return ""
}

func (idp *runtimeFirstPartyIdP) updateRememberedIdentifierCookie(writer http.ResponseWriter, identifier string, remember bool) {
	name := idp.loginHintCookieName()
	if !remember {
		http.SetCookie(writer, &http.Cookie{
			Name: name, Value: "", Path: "/idp", MaxAge: -1, Expires: time.Unix(0, 0),
			Secure: idp.config.CookieSecure, HttpOnly: false, SameSite: http.SameSiteLaxMode,
		})
		return
	}
	_, normalized, err := accesspersistence.NormalizeLoginIdentifier(identifier)
	if err != nil {
		return
	}
	cookie := &http.Cookie{
		Name: name, Value: url.QueryEscape(normalized), Path: "/idp",
		Secure: idp.config.CookieSecure, HttpOnly: false, SameSite: http.SameSiteLaxMode,
	}
	if idp.config.RememberIdentifierTTL > 0 {
		cookie.MaxAge = int(idp.config.RememberIdentifierTTL.Seconds())
		cookie.Expires = time.Now().UTC().Add(idp.config.RememberIdentifierTTL)
	}
	http.SetCookie(writer, cookie)
}

func (idp *runtimeFirstPartyIdP) loginHintCookieName() string {
	if idp.config.CookieSecure {
		return "__Host-biz-login-hint"
	}
	return "biz_login_hint"
}
