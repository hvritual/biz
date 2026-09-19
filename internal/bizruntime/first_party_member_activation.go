package bizruntime

import (
	"errors"
	"html/template"
	"net/http"
	"strings"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

type firstPartyMemberActivationPage struct {
	Token            string
	Message          string
	Success          bool
	RequiresPassword bool
}

func (idp *runtimeFirstPartyIdP) handleMemberActivationPage(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	token := strings.TrimSpace(request.URL.Query().Get("token"))
	view, err := store.InspectMemberActivation(request.Context(), token)
	if err != nil {
		status := http.StatusUnauthorized
		message := "激活链接无效。"
		switch {
		case errors.Is(err, accesspersistence.ErrMemberActivationExpired):
			status, message = http.StatusGone, "激活链接已过期。"
		case errors.Is(err, accesspersistence.ErrMemberActivationConsumed):
			status, message = http.StatusConflict, "该激活链接已使用。"
		}
		idp.renderMemberActivation(writer, status, firstPartyMemberActivationPage{Token: token, Message: message})
		return
	}
	idp.renderMemberActivation(writer, http.StatusOK, firstPartyMemberActivationPage{
		Token: token, RequiresPassword: view.RequiresPassword,
	})
}

func (idp *runtimeFirstPartyIdP) handleMemberActivationComplete(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 32<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid activation request", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(request.Form.Get("token"))
	view, inspectErr := store.InspectMemberActivation(request.Context(), token)
	if inspectErr != nil {
		idp.renderMemberActivation(writer, http.StatusUnauthorized, firstPartyMemberActivationPage{Token: token, Message: "激活链接无效。"})
		return
	}
	newPassword := request.Form.Get("new_password")
	confirmation := request.Form.Get("confirm_password")
	_, err := store.CompleteMemberActivationLink(request.Context(), token, newPassword, confirmation)
	if err != nil {
		status := http.StatusBadRequest
		message := "激活失败，请重新检查输入。"
		switch {
		case errors.Is(err, accesspersistence.ErrMemberActivationExpired):
			status, message = http.StatusGone, "激活链接已过期。"
		case errors.Is(err, accesspersistence.ErrMemberActivationConsumed):
			status, message = http.StatusConflict, "该激活链接已使用。"
		case errors.Is(err, accesspersistence.ErrWeakUserPassword), errors.Is(err, accesspersistence.ErrPasswordMismatch):
			status, message = http.StatusUnprocessableEntity, "密码需为 8–16 位，至少包含一个大写字母和一个数字，且两次输入一致。"
		}
		idp.renderMemberActivation(writer, status, firstPartyMemberActivationPage{
			Token: token, RequiresPassword: view.RequiresPassword, Message: message,
		})
		return
	}
	idp.renderMemberActivation(writer, http.StatusOK, firstPartyMemberActivationPage{
		Success: true, Message: "企业成员已激活，请返回 CoffeeLink 登录。",
	})
}

func (idp *runtimeFirstPartyIdP) renderMemberActivation(writer http.ResponseWriter, status int, page firstPartyMemberActivationPage) {
	idp.setLoginSecurityHeaders(writer, "")
	writer.WriteHeader(status)
	_ = firstPartyMemberActivationTemplate.Execute(writer, page)
}

type firstPartyInitialPasswordPage struct {
	RequestID  string
	CSRF       string
	Identifier string
	Remember   bool
	Message    string
}

func (idp *runtimeFirstPartyIdP) renderInitialPasswordChange(writer http.ResponseWriter, status int, page firstPartyInitialPasswordPage) {
	idp.setLoginSecurityHeaders(writer, "")
	writer.WriteHeader(status)
	_ = firstPartyInitialPasswordTemplate.Execute(writer, page)
}

func (idp *runtimeFirstPartyIdP) handleInitialPasswordChange(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 32<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid password change request", http.StatusBadRequest)
		return
	}
	requestID := strings.TrimSpace(request.Form.Get("request_id"))
	csrf := strings.TrimSpace(request.Form.Get("csrf_token"))
	identifier := strings.TrimSpace(request.Form.Get("identifier"))
	newPassword := request.Form.Get("new_password")
	confirmation := request.Form.Get("confirm_password")
	remember := request.Form.Get("remember_identifier") == "true"
	cookie, err := request.Cookie(idp.authCookieName())
	if err != nil || requestID == "" || csrf == "" || strings.TrimSpace(cookie.Value) == "" {
		http.Error(writer, "invalid password change request", http.StatusUnauthorized)
		return
	}
	identity, auditID, err := store.CompleteInitialPasswordActivationForLogin(
		request.Context(), requestID, cookie.Value, csrf, newPassword, confirmation,
	)
	if err != nil {
		status := http.StatusUnauthorized
		message := "一次性密码会话已失效，请重新登录。"
		switch {
		case errors.Is(err, accesspersistence.ErrWeakUserPassword), errors.Is(err, accesspersistence.ErrPasswordMismatch):
			status, message = http.StatusUnprocessableEntity, "新密码需为 8–16 位，至少包含一个大写字母和一个数字，且两次输入一致。"
		case errors.Is(err, accesspersistence.ErrMemberActivationExpired):
			status, message = http.StatusGone, "一次性初始密码已过期，请联系管理员重新发起激活。"
		}
		idp.renderInitialPasswordChange(writer, status, firstPartyInitialPasswordPage{
			RequestID: requestID, CSRF: csrf, Identifier: identifier, Remember: remember, Message: message,
		})
		return
	}
	idp.continueVerifiedLogin(writer, request, requestID, csrf, cookie.Value, identifier, remember, identity, auditID)
}

var firstPartyMemberActivationTemplate = template.Must(template.New("member-activation").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>CoffeeLink 成员激活</title>
<style>:root{font-family:Inter,"PingFang SC","Microsoft YaHei",sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center}main{width:min(460px,calc(100vw - 32px));background:#fff;border:1px solid #e7eaf0;border-radius:16px;padding:32px}h1{font-size:24px;margin:0 0 8px}.field{display:grid;gap:7px;margin:16px 0}input{width:100%;border:1px solid #d9dee8;border-radius:8px;padding:11px 12px}button{width:100%;border:0;border-radius:8px;background:#2563eb;color:#fff;padding:12px;font-weight:700}.alert{padding:10px 12px;border-radius:8px;background:#fff1eb;color:#b53c00;margin:16px 0}.success{background:#ecfdf3;color:#166534}</style></head>
<body><main><h1>企业成员激活</h1>
{{if .Message}}<div class="alert {{if .Success}}success{{end}}" role="alert">{{.Message}}</div>{{end}}
{{if .Success}}<p>激活完成。请关闭本页并从 CoffeeLink 登录。</p>
{{else}}<form method="post" action="/idp/member/activate">
<input type="hidden" name="token" value="{{.Token}}">
{{if .RequiresPassword}}
<label class="field">设置登录密码<input type="password" name="new_password" minlength="8" maxlength="16" required autocomplete="new-password"></label>
<label class="field">确认密码<input type="password" name="confirm_password" minlength="8" maxlength="16" required autocomplete="new-password"></label>
<p>密码需 8–16 位，并至少包含一个大写字母和一个数字。</p>
{{else}}<p>确认后将为当前账号开通该企业成员关系，不会修改你已有的全局登录密码。</p>{{end}}
<button type="submit">确认激活</button></form>{{end}}
</main></body></html>`))

var firstPartyInitialPasswordTemplate = template.Must(template.New("initial-password-change").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>CoffeeLink 设置新密码</title>
<style>:root{font-family:Inter,"PingFang SC","Microsoft YaHei",sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center}main{width:min(460px,calc(100vw - 32px));background:#fff;border:1px solid #e7eaf0;border-radius:16px;padding:32px}.field{display:grid;gap:7px;margin:16px 0}input{width:100%;border:1px solid #d9dee8;border-radius:8px;padding:11px 12px}button{width:100%;border:0;border-radius:8px;background:#2563eb;color:#fff;padding:12px;font-weight:700}.alert{padding:10px 12px;border-radius:8px;background:#fff1eb;color:#b53c00;margin:16px 0}</style></head>
<body><main><h1>首次登录需要修改密码</h1><p>一次性初始密码仅用于本次身份验证，设置新密码后才会激活企业成员关系。</p>
{{if .Message}}<div class="alert" role="alert">{{.Message}}</div>{{end}}
<form method="post" action="/idp/initial-password/change">
<input type="hidden" name="request_id" value="{{.RequestID}}">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<input type="hidden" name="identifier" value="{{.Identifier}}">
<input type="hidden" name="remember_identifier" value="{{if .Remember}}true{{else}}false{{end}}">
<label class="field">新密码<input type="password" name="new_password" minlength="8" maxlength="16" required autocomplete="new-password"></label>
<label class="field">确认新密码<input type="password" name="confirm_password" minlength="8" maxlength="16" required autocomplete="new-password"></label>
<button type="submit">设置密码并继续</button></form></main></body></html>`))
