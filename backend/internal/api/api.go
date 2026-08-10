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

type enabledRequest struct {
	Enabled bool `json:"enabled"`
}

type authorizeRequest struct {
	ClientID    string `json:"clientId"`
	RedirectURI string `json:"redirectUri"`
	State       string `json:"state"`
	Username    string `json:"username"`
	Password    string `json:"password"`
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
	api.PUT("/profile", a.requireCSRF(), a.updateMyProfile)
	api.GET("/employees", a.listEmployees)
	api.POST("/employees", a.requireCSRF(), a.createEmployee)
	api.PUT("/employees/:id", a.requireCSRF(), a.updateEmployee)
	api.DELETE("/employees/:id", a.requireCSRF(), a.deleteEmployee)
	api.POST("/employees/:id/reset-password", a.requireCSRF(), a.resetEmployeePassword)
	api.PUT("/employees/:id/enabled", a.requireCSRF(), a.setEmployeeEnabled)
	api.GET("/employees/:id/events", a.listEmploymentEvents)
	api.GET("/departments", a.listDepartments)
	api.POST("/departments", a.requireCSRF(), a.createDepartment)
	api.PUT("/departments/:id", a.requireCSRF(), a.updateDepartment)
	api.DELETE("/departments/:id", a.requireCSRF(), a.deleteDepartment)
	api.GET("/positions", a.listPositions)
	api.POST("/positions", a.requireCSRF(), a.createPosition)
	api.PUT("/positions/:id", a.requireCSRF(), a.updatePosition)
	api.DELETE("/positions/:id", a.requireCSRF(), a.deletePosition)
	api.GET("/hr/dashboard", a.hrDashboard)
	api.GET("/approval-types", a.listApprovalTypes)
	api.GET("/approvals", a.listApprovals)
	api.POST("/approvals", a.requireCSRF(), a.createApproval)
	api.GET("/approvals/:id", a.getApproval)
	api.POST("/approvals/:id/review", a.requireCSRF(), a.reviewApproval)
	api.POST("/approvals/:id/cancel", a.requireCSRF(), a.cancelApproval)
	api.GET("/leave/balance", a.leaveBalance)
	api.GET("/leave/calendar", a.leaveCalendar)
	api.GET("/contracts", a.listContracts)
	api.POST("/employees/:id/contracts", a.requireCSRF(), a.createContract)
	api.PUT("/contracts/:id", a.requireCSRF(), a.updateContract)
	api.DELETE("/contracts/:id", a.requireCSRF(), a.deleteContract)
	api.GET("/performance-goals", a.listGoals)
	api.POST("/performance-goals", a.requireCSRF(), a.createGoal)
	api.PUT("/performance-goals/:id", a.requireCSRF(), a.updateGoal)
	api.GET("/departures", a.listDepartures)
	api.POST("/departures", a.requireCSRF(), a.createDeparture)
	api.POST("/departures/:id/manager-review", a.requireCSRF(), a.reviewDeparture("manager"))
	api.POST("/departures/:id/hr-review", a.requireCSRF(), a.reviewDeparture("hr"))
	api.POST("/departures/:id/cancel", a.requireCSRF(), a.cancelDeparture)
	api.GET("/notifications", a.listNotifications)
	api.GET("/notifications/summary", a.notificationSummary)
	api.POST("/notifications/:id/read", a.requireCSRF(), a.markNotificationRead)
	api.POST("/notifications/read-all", a.requireCSRF(), a.markAllNotificationsRead)
	api.POST("/oauth/authorize", a.requireCSRF(), a.authorize)
	api.POST("/oauth/token", a.token)
	api.GET("/oauth/userinfo", a.userinfo)

	inner := h.Group("/api/v1/inner")
	inner.Use(innerVerifier.Middleware())
	inner.GET("/directory/employees", a.listDirectoryEmployees)
	inner.GET("/directory/employees/:id", a.getDirectoryEmployee)
	inner.GET("/directory/departments", a.listDirectoryDepartments)
	inner.GET("/directory/positions", a.listDirectoryPositions)
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
	} else if _, _, valid := a.permissionSession(c, service.PermissionEmployeeView); !valid {
		return
	}
	result, err := a.service.ListEmployees(string(c.Query("q")), queryInt(c, "page", 1), queryInt(c, "pageSize", 20))
	respond(c, result, err)
}

func (a *API) createEmployee(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionEmployeeManage); !valid {
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
	if _, _, valid := a.permissionSession(c, service.PermissionEmployeeManage); !valid {
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
	employee, _, valid := a.permissionSession(c, service.PermissionEmployeeManage)
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
	if _, _, valid := a.session(c, false); !valid {
		return
	}
	result, err := a.service.ListDepartments(string(c.Query("q")))
	respond(c, result, err)
}

func (a *API) createDepartment(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionDepartmentManage); !valid {
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
	if _, _, valid := a.permissionSession(c, service.PermissionDepartmentManage); !valid {
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
	if _, _, valid := a.permissionSession(c, service.PermissionDepartmentManage); !valid {
		return
	}
	if err := a.service.DeleteDepartment(c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"deleted": true})
}

func (a *API) listPositions(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.session(c, false); !valid {
		return
	}
	result, err := a.service.ListPositions(string(c.Query("q")), string(c.Query("departmentId")))
	respond(c, result, err)
}

func (a *API) createPosition(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionDepartmentManage); !valid {
		return
	}
	var request service.PositionInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreatePosition(request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) updatePosition(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionDepartmentManage); !valid {
		return
	}
	var request service.PositionInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.UpdatePosition(c.Param("id"), request)
	respond(c, result, err)
}

func (a *API) deletePosition(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionDepartmentManage); !valid {
		return
	}
	if err := a.service.DeletePosition(c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"deleted": true})
}

func (a *API) resetEmployeePassword(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionEmployeeReset); !valid {
		return
	}
	if err := a.service.ResetEmployeePassword(c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"reset": true})
}

func (a *API) setEmployeeEnabled(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionEmployeeDisable); !valid {
		return
	}
	var request enabledRequest
	if !bind(c, &request) {
		return
	}
	result, err := a.service.SetEmployeeEnabled(c.Param("id"), request.Enabled)
	respond(c, result, err)
}

func (a *API) updateMyProfile(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request service.ProfileInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.UpdateMyProfile(employee, request)
	respond(c, result, err)
}

func (a *API) listEmploymentEvents(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	publicID := c.Param("id")
	if publicID != employee.PublicID && !a.service.HasPermission(employee, service.PermissionEmployeeView) {
		handleError(c, service.ErrForbidden)
		return
	}
	result, err := a.service.ListEmploymentEvents(publicID)
	respond(c, result, err)
}

func (a *API) hrDashboard(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionDashboardView); !valid {
		return
	}
	result, err := a.service.Dashboard()
	respond(c, result, err)
}

func (a *API) listApprovalTypes(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.session(c, false); !valid {
		return
	}
	ok(c, service.ApprovalTypes())
}

func (a *API) listApprovals(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	result, err := a.service.ListApprovals(employee, service.ApprovalFilter{
		Scope: string(c.Query("scope")), Type: string(c.Query("type")), Status: string(c.Query("status")),
	}, a.service.HasPermission(employee, service.PermissionApprovalView), a.service.HasPermission(employee, service.PermissionApprovalReview))
	respond(c, result, err)
}

func (a *API) createApproval(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request service.ApprovalInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreateApproval(employee, request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) getApproval(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	result, err := a.service.GetApproval(employee, c.Param("id"), a.service.HasPermission(employee, service.PermissionApprovalView), a.service.HasPermission(employee, service.PermissionApprovalReview))
	respond(c, result, err)
}

func (a *API) reviewApproval(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request service.ReviewInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.ReviewApproval(employee, c.Param("id"), request, a.service.HasPermission(employee, service.PermissionApprovalReview))
	respond(c, result, err)
}

func (a *API) cancelApproval(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	if err := a.service.CancelApproval(employee, c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"cancelled": true})
}

func (a *API) leaveBalance(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	result, err := a.service.GetLeaveBalance(employee, queryInt(c, "year", time.Now().UTC().Year()))
	respond(c, result, err)
}

func (a *API) leaveCalendar(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	result, err := a.service.ListLeaveCalendar(employee, a.service.HasPermission(employee, service.PermissionApprovalView), string(c.Query("month")))
	respond(c, result, err)
}

func (a *API) listContracts(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	canManage := a.service.HasPermission(employee, service.PermissionContractManage)
	canView := canManage || a.service.HasPermission(employee, service.PermissionContractView)
	result, err := a.service.ListContracts(employee, canView, string(c.Query("employeeId")))
	respond(c, result, err)
}

func (a *API) createContract(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionContractManage); !valid {
		return
	}
	var request service.ContractInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreateContract(c.Param("id"), request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) updateContract(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionContractManage); !valid {
		return
	}
	var request service.ContractInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.UpdateContract(c.Param("id"), request)
	respond(c, result, err)
}

func (a *API) deleteContract(_ context.Context, c *app.RequestContext) {
	if _, _, valid := a.permissionSession(c, service.PermissionContractManage); !valid {
		return
	}
	if err := a.service.DeleteContract(c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"deleted": true})
}

func (a *API) listGoals(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	canViewAll := a.service.HasPermission(employee, service.PermissionPerformanceView) || a.service.HasPermission(employee, service.PermissionPerformanceManage)
	result, err := a.service.ListGoals(employee, canViewAll, string(c.Query("cycle")))
	respond(c, result, err)
}

func (a *API) createGoal(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request service.GoalInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreateGoal(employee, request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) updateGoal(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request service.GoalInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.UpdateGoal(employee, c.Param("id"), request, a.service.HasPermission(employee, service.PermissionPerformanceManage))
	respond(c, result, err)
}

func (a *API) listDepartures(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	canReview := a.service.HasPermission(employee, service.PermissionDepartureReview) || a.service.HasPermission(employee, service.PermissionApprovalReview)
	result, err := a.service.ListDepartures(employee, canReview)
	respond(c, result, err)
}

func (a *API) createDeparture(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	var request service.DepartureInput
	if !bind(c, &request) {
		return
	}
	result, err := a.service.CreateDeparture(employee, request)
	if err != nil {
		handleError(c, err)
		return
	}
	created(c, result)
}

func (a *API) reviewDeparture(stage string) app.HandlerFunc {
	return func(_ context.Context, c *app.RequestContext) {
		employee, _, valid := a.session(c, false)
		if !valid {
			return
		}
		var request service.ReviewInput
		if !bind(c, &request) {
			return
		}
		canReview := a.service.HasPermission(employee, service.PermissionDepartureReview) || a.service.HasPermission(employee, service.PermissionApprovalReview)
		result, err := a.service.ReviewDeparture(employee, c.Param("id"), stage, request, canReview)
		respond(c, result, err)
	}
}

func (a *API) cancelDeparture(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	if err := a.service.CancelDeparture(employee, c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"cancelled": true})
}

func (a *API) listNotifications(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	result, err := a.service.ListNotifications(employee.PublicID, string(c.Query("unread")) == "true")
	respond(c, result, err)
}

func (a *API) notificationSummary(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	canReview := a.service.HasPermission(employee, service.PermissionDepartureReview) || a.service.HasPermission(employee, service.PermissionApprovalReview)
	result, err := a.service.NotificationSummary(employee, canReview)
	respond(c, result, err)
}

func (a *API) markNotificationRead(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	if err := a.service.MarkNotificationRead(employee.PublicID, c.Param("id")); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"read": true})
}

func (a *API) markAllNotificationsRead(_ context.Context, c *app.RequestContext) {
	employee, _, valid := a.session(c, false)
	if !valid {
		return
	}
	if err := a.service.MarkAllNotificationsRead(employee.PublicID); err != nil {
		handleError(c, err)
		return
	}
	ok(c, utils.H{"read": true})
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

func (a *API) listDirectoryPositions(_ context.Context, c *app.RequestContext) {
	result, err := a.service.ListPositions(string(c.Query("q")), string(c.Query("departmentId")))
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
	if strings.TrimSpace(request.Username) != "" || request.Password != "" {
		if strings.TrimSpace(request.Username) == "" || request.Password == "" {
			handleError(c, fmt.Errorf("%w: 切换账号时用户名和密码不能为空", service.ErrInvalid))
			return
		}
		var err error
		employee, err = a.service.AuthenticateOAuthAccount(request.Username, request.Password)
		if err != nil {
			handleError(c, err)
			return
		}
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

func (a *API) permissionSession(c *app.RequestContext, code string) (*model.Employee, *model.Session, bool) {
	employee, session, valid := a.session(c, false)
	if !valid {
		return nil, nil, false
	}
	if !a.service.HasPermission(employee, code) {
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
