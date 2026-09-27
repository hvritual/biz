package bizruntime

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

type firstPartyRecoveryPage struct {
	Stage        string
	FlowID       string
	RequestNonce string
	Identifier   string
	ChallengeID  string
	Message      string
	Success      bool
	OTPEnabled   bool
	CodeDigits   int
	LoginURL     string
}

func (idp *runtimeFirstPartyIdP) handlePasswordRecoveryPage(writer http.ResponseWriter, request *http.Request) {
	flowID, err := randomURLSecret(24)
	if err != nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	idp.setRecoveryCookie(writer, flowID)
	idp.renderPasswordRecovery(writer, http.StatusOK, firstPartyRecoveryPage{Stage: "request", FlowID: flowID})
}

func (idp *runtimeFirstPartyIdP) handlePasswordRecoveryRequest(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	verification := idp.currentVerification()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid recovery request", http.StatusBadRequest)
		return
	}
	identifier := strings.TrimSpace(request.Form.Get("identifier"))
	flowID := strings.TrimSpace(request.Form.Get("flow_id"))
	requestNonce := strings.TrimSpace(request.Form.Get("request_nonce"))
	if identifier == "" || flowID == "" || requestNonce == "" || !idp.validRecoveryCookie(request, flowID) {
		idp.renderPasswordRecovery(writer, http.StatusBadRequest, firstPartyRecoveryPage{
			Stage: "request", FlowID: flowID, Identifier: identifier, Message: "请输入有效账号、手机号或邮箱。",
		})
		return
	}
	if verification == nil || idp.currentVerificationProtection() == nil || idp.config.RecoveryAuthorizationTTL <= 0 {
		idp.renderPasswordRecovery(writer, http.StatusServiceUnavailable, firstPartyRecoveryPage{
			Stage: "request", FlowID: flowID, Identifier: identifier, Message: "找回密码当前未配置可用验证渠道。",
		})
		return
	}
	resolved, resolveErr := store.ResolveLoginIdentifier(request.Context(), identifier)
	if resolveErr != nil {
		fake, _ := randomURLSecret(24)
		idp.renderPasswordRecovery(writer, http.StatusOK, firstPartyRecoveryPage{
			Stage: "complete", FlowID: flowID, Identifier: identifier, ChallengeID: "vch-fake-" + fake,
			Message: "找回请求已受理；若账号可用且渠道正常，将发送验证码。",
		})
		return
	}
	challenge, _, _ := verification.SendVerificationCode(request.Context(), accessdomain.VerificationChallengeRequest{
		BusinessEventID: "password-recovery/" + flowID + "/" + accesspersistence.TokenHash(requestNonce),
		FlowID:          flowID,
		Purpose:         accessdomain.VerificationPurposePasswordRecovery,
		UserID:          resolved.Identity.UserID,
		Channel:         resolved.OTPChannel,
		Destination:     resolved.OTPDestination,
	})
	if challenge.ChallengeID == "" {
		fake, _ := randomURLSecret(24)
		challenge.ChallengeID = "vch-fake-" + fake
	}
	idp.renderPasswordRecovery(writer, http.StatusOK, firstPartyRecoveryPage{
		Stage: "complete", FlowID: flowID, Identifier: identifier, ChallengeID: challenge.ChallengeID,
		Message: "找回请求已受理；若账号可用且渠道正常，将发送验证码。",
	})
}

func (idp *runtimeFirstPartyIdP) handlePasswordRecoveryComplete(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	verification := idp.currentVerification()
	protection := idp.currentVerificationProtection()
	if store == nil || verification == nil || protection == nil || idp.config.RecoveryAuthorizationTTL <= 0 {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 32<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid recovery request", http.StatusBadRequest)
		return
	}
	identifier := strings.TrimSpace(request.Form.Get("identifier"))
	flowID := strings.TrimSpace(request.Form.Get("flow_id"))
	challengeID := strings.TrimSpace(request.Form.Get("challenge_id"))
	code := strings.TrimSpace(request.Form.Get("otp_code"))
	newPassword := request.Form.Get("new_password")
	confirmation := request.Form.Get("confirm_password")
	page := firstPartyRecoveryPage{Stage: "complete", FlowID: flowID, Identifier: identifier, ChallengeID: challengeID}
	if !idp.validRecoveryCookie(request, flowID) {
		http.Error(writer, "invalid recovery flow", http.StatusUnauthorized)
		return
	}
	if newPassword != confirmation {
		page.Message = "两次输入的新密码不一致。"
		idp.renderPasswordRecovery(writer, http.StatusUnprocessableEntity, page)
		return
	}
	if err := accesspersistence.ValidateUserChosenPassword(newPassword); err != nil {
		page.Message = "新密码需为 8–16 位，并至少包含一个大写字母和一个数字。"
		idp.renderPasswordRecovery(writer, http.StatusUnprocessableEntity, page)
		return
	}
	resolved, err := store.ResolveLoginIdentifier(request.Context(), identifier)
	if err != nil || strings.HasPrefix(challengeID, "vch-fake-") {
		page.Message = "账号或验证码错误。"
		idp.renderPasswordRecovery(writer, http.StatusUnauthorized, page)
		return
	}
	_, err = store.RecoverPasswordWithCode(request.Context(), protection, accessdomain.VerifyChallengeRequest{
		ChallengeID: challengeID,
		FlowID:      flowID,
		Purpose:     accessdomain.VerificationPurposePasswordRecovery,
		UserID:      resolved.Identity.UserID,
		Channel:     resolved.OTPChannel,
		Destination: resolved.OTPDestination,
		Code:        code,
	}, idp.config.RecoveryAuthorizationTTL, newPassword, confirmation)
	if err != nil {
		switch {
		case errors.Is(err, accessdomain.ErrVerificationExpired):
			page.Message = "验证码已过期，请重新发起找回。"
		case errors.Is(err, accessdomain.ErrVerificationConsumed):
			page.Message = "验证码已使用，请重新发起找回。"
		case errors.Is(err, accessdomain.ErrVerificationRateLimited):
			page.Message = "验证码错误次数过多，请重新发起找回。"
		default:
			page.Message = "账号或验证码错误。"
		}
		idp.renderPasswordRecovery(writer, http.StatusUnauthorized, page)
		return
	}
	idp.clearRecoveryCookie(writer)
	idp.renderPasswordRecovery(writer, http.StatusOK, firstPartyRecoveryPage{
		Stage: "done", Success: true, Message: "密码已更新，请使用新密码重新登录。",
	})
}

func (idp *runtimeFirstPartyIdP) recoveryCookieName() string {
	if idp.config.CookieSecure {
		return "__Host-biz-idp-recovery"
	}
	return "biz_idp_recovery"
}

func (idp *runtimeFirstPartyIdP) setRecoveryCookie(writer http.ResponseWriter, flowID string) {
	http.SetCookie(writer, &http.Cookie{
		Name: idp.recoveryCookieName(), Value: flowID, Path: "/idp/password/recovery",
		HttpOnly: true, Secure: idp.config.CookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: int((10 * time.Minute).Seconds()), Expires: time.Now().UTC().Add(10 * time.Minute),
	})
}

func (idp *runtimeFirstPartyIdP) clearRecoveryCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name: idp.recoveryCookieName(), Value: "", Path: "/idp/password/recovery",
		HttpOnly: true, Secure: idp.config.CookieSecure, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0).UTC(),
	})
}

func (idp *runtimeFirstPartyIdP) validRecoveryCookie(request *http.Request, flowID string) bool {
	cookie, err := request.Cookie(idp.recoveryCookieName())
	if err != nil || strings.TrimSpace(flowID) == "" {
		return false
	}
	return constantTimeEqual(cookie.Value, flowID)
}

func (idp *runtimeFirstPartyIdP) renderPasswordRecovery(writer http.ResponseWriter, status int, page firstPartyRecoveryPage) {
	page.OTPEnabled = idp.currentVerification() != nil && idp.currentVerificationProtection() != nil && idp.config.RecoveryAuthorizationTTL > 0
	page.CodeDigits = idp.config.OTPCodeDigits
	if page.CodeDigits == 0 {
		page.CodeDigits = 6
	}
	if page.RequestNonce == "" {
		nonce, err := randomURLSecret(18)
		if err != nil {
			http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
			return
		}
		page.RequestNonce = nonce
	}
	redirect, err := url.Parse(idp.config.RedirectURL)
	if err == nil && redirect.Scheme != "" && redirect.Host != "" {
		page.LoginURL = redirect.Scheme + "://" + redirect.Host + "/auth/login?return_to=/auth/session"
	}
	if page.LoginURL == "" {
		page.LoginURL = "/"
	}
	scriptNonce, err := randomURLSecret(18)
	if err != nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	idp.setLoginSecurityHeaders(writer, scriptNonce)
	writer.WriteHeader(status)
	_ = firstPartyRecoveryTemplate.Execute(writer, struct {
		firstPartyRecoveryPage
		ScriptNonce string
	}{firstPartyRecoveryPage: page, ScriptNonce: scriptNonce})
}

var firstPartyRecoveryTemplate = template.Must(template.New("first-party-password-recovery").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>CoffeeLink 找回密码</title>
<style>
:root{font-family:Inter,"PingFang SC","Microsoft YaHei",sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f5f7fb}main{width:min(460px,calc(100vw - 32px));background:#fff;border:1px solid #e7eaf0;border-radius:16px;padding:32px;box-shadow:0 20px 60px rgba(22,34,51,.10)}h1{font-size:24px;margin:0 0 8px}p{color:#6b7280;font-size:13px;line-height:1.7}.field{display:grid;gap:7px;margin:16px 0}label{font-size:13px;font-weight:600}input{width:100%;border:1px solid #d9dee8;border-radius:8px;padding:11px 12px;font:inherit}button,.link{display:inline-flex;justify-content:center;width:100%;margin-top:14px;border:0;border-radius:8px;background:#fc610e;color:white;padding:12px;font:inherit;font-weight:700;text-decoration:none;cursor:pointer}.secondary{background:#fff;color:#596273;border:1px solid #d9dee8}.alert{padding:10px 12px;border-radius:8px;background:#fff1eb;color:#b53c00;font-size:13px;margin:16px 0}.success{background:#ecfdf3;color:#166534}.password-row{display:grid;grid-template-columns:1fr auto;gap:8px}.toggle{width:auto;margin:0;background:#fff;color:#596273;border:1px solid #d9dee8;padding:0 12px}
</style>
</head>
<body><main>
<h1>找回密码</h1>
<p>通过已绑定的安全联系方式验证身份。未注册或存在歧义的账号不会返回可区分的存在性信息。</p>
{{if .Message}}<div class="alert {{if .Success}}success{{end}}" role="alert">{{.Message}}</div>{{end}}
{{if eq .Stage "request"}}
<form method="post" action="/idp/password/recovery/request">
<input type="hidden" name="flow_id" value="{{.FlowID}}">
<input type="hidden" name="request_nonce" value="{{.RequestNonce}}">
<label class="field">账号 / 手机号 / 邮箱<input name="identifier" aria-label="找回密码账号 / 手机号 / 邮箱" value="{{.Identifier}}" required maxlength="320"></label>
<button type="submit" {{if not .OTPEnabled}}disabled{{end}}>发送验证码</button>
</form>
{{else if eq .Stage "complete"}}
<form method="post" action="/idp/password/recovery/complete">
<input type="hidden" name="flow_id" value="{{.FlowID}}">
<input type="hidden" name="challenge_id" value="{{.ChallengeID}}">
<input type="hidden" name="identifier" value="{{.Identifier}}">
<label class="field">验证码<input name="otp_code" aria-label="找回密码验证码" inputmode="numeric" autocomplete="one-time-code" minlength="{{.CodeDigits}}" maxlength="{{.CodeDigits}}" required></label>
<label class="field">新密码<div class="password-row"><input id="recovery-new-password" name="new_password" aria-label="找回密码新密码" type="password" minlength="8" maxlength="16" required><button class="toggle" type="button" data-toggle="recovery-new-password">显示</button></div></label>
<label class="field">确认新密码<div class="password-row"><input id="recovery-confirm-password" name="confirm_password" aria-label="找回密码确认新密码" type="password" minlength="8" maxlength="16" required><button class="toggle" type="button" data-toggle="recovery-confirm-password">显示</button></div></label>
<p>密码需 8–16 位，并至少包含一个大写字母和一个数字。</p>
<button type="submit">确认修改密码</button>
</form>
{{else}}
<a class="link" href="{{.LoginURL}}">返回登录</a>
{{end}}
{{if ne .Stage "done"}}<a class="link secondary" href="{{.LoginURL}}">返回登录</a>{{end}}
<script nonce="{{.ScriptNonce}}">
for (const button of document.querySelectorAll('[data-toggle]')) {
  button.addEventListener('click', () => {
    const input = document.getElementById(button.dataset.toggle)
    if (!input) return
    input.type = input.type === 'password' ? 'text' : 'password'
    button.textContent = input.type === 'password' ? '显示' : '隐藏'
  })
}
</script>
</main></body></html>`))
