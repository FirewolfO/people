package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"people/internal/model"
	"people/internal/security"
	"people/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol"
)

const (
	sessionCookie = "PEOPLE_SESSION"
	csrfCookie    = "PEOPLE_XSRF"
	requestIDKey  = "requestID"
)

var requestSequence uint64

type API struct {
	service      *service.Service
	sessionTTL   time.Duration
	cookieSecure bool
}

type response struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"requestId"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type authorizeRequest struct {
	ClientID    string `json:"clientId"`
	RedirectURI string `json:"redirectUri"`
	State       string `json:"state"`
}

type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	Scope        string `json:"scope"`
}

func NewServer(address string, verifier, innerVerifier *security.GatewayVerifier, svc *service.Service, sessionTTL time.Duration, cookieSecure bool) *server.Hertz {
	h := server.Default(server.WithHostPorts(address))
	a := &API{service: svc, sessionTTL: sessionTTL, cookieSecure: cookieSecure}
	h.Use(requestIDMiddleware())
	h.GET("/health", a.health)

	api := h.Group("/api/v1")
	api.Use(verifier.Middleware())
	api.GET("/auth/csrf", a.csrf)
	api.POST("/auth/login", a.requireCSRF(), a.login)
	api.GET("/auth/me", a.me)
	api.POST("/auth/logout", a.requireCSRF(), a.logout)
	api.POST("/auth/change-password", a.requireCSRF(), a.changePassword)
	api.GET("/employees", a.listEmployees)
	api.POST("/employees", a.requireCSRF(), a.createEmployee)
	api.PUT("/employees/:id", a.requireCSRF(), a.updateEmployee)
	api.DELETE("/employees/:id", a.requireCSRF(), a.deleteEmployee)
	api.GET("/departments", a.listDepartments)
	api.POST("/departments", a.requireCSRF(), a.createDepartment)
	api.PUT("/departments/:id", a.requireCSRF(), a.updateDepartment)
	api.DELETE("/departments/:id", a.requireCSRF(), a.deleteDepartment)
	api.POST("/oauth/authorize", a.requireCSRF(), a.authorize)
	api.POST("/oauth/token", a.token)
	api.GET("/oauth/userinfo", a.userinfo)

	inner := h.Group("/api/v1/inner")
	inner.Use(innerVerifier.Middleware())
	inner.GET("/directory/employees", a.listDirectoryEmployees)
	inner.GET("/directory/employees/:id", a.getDirectoryEmployee)
	inner.GET("/directory/departments", a.listDirectoryDepartments)
	return h
}

func (a *API) health(_ context.Context, c *app.RequestContext) {
	ok(c, utils.H{"status": "ok", "time": time.Now().UTC()})
}

func (a *API) csrf(_ context.Context, c *app.RequestContext) {
	token, err := randomValue(32)
	if err != nil {
		handleError(c, err)
		return
	}
	c.SetCookie(csrfCookie, token, int(a.sessionTTL.Seconds()), "/", "", protocol.CookieSameSiteLaxMode, a.cookieSecure, false)
	ok(c, utils.H{"token": token, "headerName": "X-XSRF-TOKEN"})
}

func (a *API) login(_ context.Context, c *app.RequestContext) {
	var request loginRequest
	if !bind(c, &request) {
		return
	}
	employee, token, err := a.service.Login(request.Username, request.Password)
	if err != nil {
		handleError(c, err)
		return
	}
	c.SetCookie(sessionCookie, token, int(a.sessionTTL.Seconds()), "/", "", protocol.CookieSameSiteLaxMode, a.cookieSecure, true)
	ok(c, employee)
}

func (a *API) me(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, true)
	if !valid {
		return
	}
	ok(c, employee)
}

func (a *API) logout(_ context.Context, c *app.RequestContext) {
	_, session, valid := a.session(c, true)
	if !valid {
		return
	}
	if err := a.service.Logout(session.ID); err != nil {
		handleError(c, err)
		return
	}
	c.SetCookie(sessionCookie, "", -1, "/", "", protocol.CookieSameSiteLaxMode, a.cookieSecure, true)
	ok(c, utils.H{"loggedOut": true})
}

func (a *API) changePassword(_ context.Context, c *app.RequestContext) {
	employee, session, valid := a.session(c, true)
	if !valid {
		return
	}
	var request passwordRequest
	if !bind(c, &request) {
		return
	}
	if err := a.service.ChangePassword(employee, session.ID, request.CurrentPassword, request.NewPassword); err != nil {
		handleError(c, err)
		return
	}
	employee.MustChangePassword = false
	ok(c, employee)
}

func (a *API) listEmployees(_ context.Context, c *app.RequestContext) {
	if authorization := bearer(c); authorization != "" {
		_, scope, err := a.service.OAuthIdentity(authorization)
		if err != nil || !hasScope(scope, "employees.read") {
			handleError(c, service.ErrForbidden)
			return
		}
	} else if _, _, valid := a.adminSession(c); !valid {
		return
	}
	result, err := a.service.ListEmployees(string(c.Query("q")), queryInt(c, "page", 1), queryInt(c, "pageSize", 20))
	respond(c, result, err)
}

func (a *API) createEmployee(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.adminSession(c); !valid {
		return
	}
	var request service.EmployeeInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreateEmployee(request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) updateEmployee(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.adminSession(c); !valid {
		return
	}
	var request service.EmployeeInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.UpdateEmployee(c.Param("id"), request)
	respond(c, result, err)
}

func (a *API) deleteEmployee(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.adminSession(c)
	if !valid {
		return
	}
	if err := a.service.DeleteEmployee(c.Param("id"), employee.ID); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"deleted": true})
}

func (a *API) listDepartments(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.adminSession(c); !valid {
		return
	}
	result, err := a.service.ListDepartments(string(c.Query("q")))
	respond(c, result, err)
}

func (a *API) createDepartment(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.adminSession(c); !valid {
		return
	}
	var request service.DepartmentInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreateDepartment(request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) updateDepartment(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.adminSession(c); !valid {
		return
	}
	var request service.DepartmentInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.UpdateDepartment(c.Param("id"), request)
	respond(c, result, err)
}

func (a *API) deleteDepartment(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.adminSession(c); !valid {
		return
	}
	if err := a.service.DeleteDepartment(c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"deleted": true})
}

func (a *API) listDirectoryEmployees(_ context.Context, c *app.RequestContext) {
	result, err := a.service.ListEmployees(string(c.Query("q")), queryInt(c, "page", 1), queryInt(c, "pageSize", 100))
	respond(c, result, err)
}

func (a *API) getDirectoryEmployee(_ context.Context, c *app.RequestContext) {
	result, err := a.service.GetEmployee(c.Param("id"))
	respond(c, result, err)
}

func (a *API) listDirectoryDepartments(_ context.Context, c *app.RequestContext) {
	result, err := a.service.ListDepartments(string(c.Query("q")))
	respond(c, result, err)
}

func (a *API) authorize(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request authorizeRequest
	if !bind(c, &request) {
		return
	}
	redirectURL, err := a.service.Authorize(employee, request.ClientID, request.RedirectURI, request.State)
	if err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"redirectUrl": redirectURL})
}

func (a *API) token(_ context.Context, c *app.RequestContext) {
	request := tokenRequest{
		GrantType: string(c.FormValue("grant_type")), ClientID: string(c.FormValue("client_id")),
		ClientSecret: string(c.FormValue("client_secret")), Code: string(c.FormValue("code")),
		RedirectURI: string(c.FormValue("redirect_uri")), Scope: string(c.FormValue("scope")),
	}
	if strings.Contains(strings.ToLower(string(c.Request.Header.ContentType())), "application/json") && !bind(c, &request) {
		return
	}
	if username, password, ok := basicAuth(c); ok {
		request.ClientID, request.ClientSecret = username, password
	}
	var result service.TokenResult
	var err error
	switch request.GrantType {
	case "authorization_code":
		result, err = a.service.ExchangeCode(request.ClientID, request.ClientSecret, request.Code, request.RedirectURI)
	case "client_credentials":
		result, err = a.service.ClientCredentials(request.ClientID, request.ClientSecret, request.Scope)
	default:
		err = fmt.Errorf("%w: 不支持的 grant_type", service.ErrInvalid)
	}
	if err != nil {
		oauthError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *API) userinfo(_ context.Context, c *app.RequestContext) {
	employee, _, err := a.service.OAuthIdentity(bearer(c))
	if err != nil || employee == nil {
		c.JSON(http.StatusUnauthorized, utils.H{"error": "invalid_token"})
		return
	}
	c.JSON(http.StatusOK, employee)
}

func (a *API) session(c *app.RequestContext, allowPasswordChange bool) (*model.Employee, *model.Session, bool) {
	employee, session, err := a.service.AuthenticateSession(string(c.Cookie(sessionCookie)))
	if err != nil {
		handleError(c, err)
		return nil, nil, false
	}
	if employee.MustChangePassword && !allowPasswordChange {
		fail(c, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "必须先设置登录密码")
		return nil, nil, false
	}
	return employee, session, true
}

func (a *API) adminSession(c *app.RequestContext) (*model.Employee, *model.Session, bool) {
	employee, session, valid := a.session(c, false)
	if !valid {
		return nil, nil, false
	}
	if employee.Role != model.RoleAdmin {
		handleError(c, service.ErrForbidden)
		return nil, nil, false
	}
	return employee, session, true
}

func (a *API) requireCSRF() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		cookie := string(c.Cookie(csrfCookie))
		header := string(c.Request.Header.Peek("X-XSRF-TOKEN"))
		if cookie == "" || header == "" || cookie != header {
			fail(c, http.StatusForbidden, "INVALID_CSRF_TOKEN", "CSRF 校验失败")
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func bind(c *app.RequestContext, value any) bool {
	if err := c.BindAndValidate(value); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_ARGUMENT", "请求参数格式错误")
		return false
	}
	return true
}

func queryInt(c *app.RequestContext, name string, fallback int) int {
	value, err := strconv.Atoi(string(c.Query(name)))
	if err != nil {
		return fallback
	}
	return value
}

func bearer(c *app.RequestContext) string {
	value := string(c.Request.Header.Peek("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func basicAuth(c *app.RequestContext) (string, string, bool) {
	value := string(c.Request.Header.Peek("Authorization"))
	if len(value) <= 6 || !strings.EqualFold(value[:6], "Basic ") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value[6:]))
	if err != nil {
		return "", "", false
	}
	username, password, found := strings.Cut(string(decoded), ":")
	return username, password, found
}

func hasScope(scopes, expected string) bool {
	for _, scope := range strings.Fields(scopes) {
		if scope == expected {
			return true
		}
	}
	return false
}

func respond(c *app.RequestContext, data any, err error) {
	if err != nil {
		handleError(c, err)
		return
	}
	ok(c, data)
}

func ok(c *app.RequestContext, data any) {
	c.JSON(http.StatusOK, response{Code: "OK", Message: "成功", Data: data, RequestID: requestID(c)})
}

func created(c *app.RequestContext, data any) {
	c.JSON(http.StatusCreated, response{Code: "OK", Message: "创建成功", Data: data, RequestID: requestID(c)})
}

func fail(c *app.RequestContext, status int, code, message string) {
	c.JSON(status, response{Code: code, Message: message, RequestID: requestID(c)})
}

func handleError(c *app.RequestContext, err error) {
	switch {
	case errors.Is(err, service.ErrInvalid):
		fail(c, http.StatusBadRequest, "INVALID_ARGUMENT", detail(err, "请求参数无效"))
	case errors.Is(err, service.ErrUnauthorized):
		fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "登录状态或凭据无效")
	case errors.Is(err, service.ErrForbidden):
		fail(c, http.StatusForbidden, "FORBIDDEN", detail(err, "无权执行此操作"))
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, "NOT_FOUND", detail(err, "资源不存在"))
	case errors.Is(err, service.ErrConflict):
		fail(c, http.StatusConflict, "CONFLICT", detail(err, "数据已存在"))
	default:
		fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误")
	}
}

func oauthError(c *app.RequestContext, err error) {
	if errors.Is(err, service.ErrUnauthorized) {
		c.JSON(http.StatusUnauthorized, utils.H{"error": "invalid_client"})
		return
	}
	c.JSON(http.StatusBadRequest, utils.H{"error": "invalid_request", "error_description": detail(err, "请求无效")})
}

func detail(err error, fallback string) string {
	parts := strings.SplitN(err.Error(), ": ", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		return parts[1]
	}
	return fallback
}

func requestIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := strings.TrimSpace(string(c.Request.Header.Peek("X-Request-ID")))
		if id == "" {
			id = fmt.Sprintf("people-%d-%d", time.Now().UTC().UnixNano(), atomic.AddUint64(&requestSequence, 1))
		}
		c.Set(requestIDKey, id)
		c.Response.Header.Set("X-Request-ID", id)
		c.Next(ctx)
	}
}

func requestID(c *app.RequestContext) string {
	return c.GetString(requestIDKey)
}

func randomValue(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
