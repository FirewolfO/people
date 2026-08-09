import axios from 'axios'
import type { ApprovalRequest, ApprovalType, ApprovalTypeDefinition, ContractInput, Department, DepartmentInput, DepartureRequest, Employee, EmployeeContract, EmployeeInput, EmploymentEvent, GoalInput, HRDashboard, LeaveBalance, LeaveRecord, NotificationItem, NotificationSummary, Page, PerformanceGoal } from '@/types'

interface Envelope<T> {
  code: string
  message: string
  data: T
  requestId: string
}

const client = axios.create({
  baseURL: import.meta.env.VITE_PEOPLE_API_BASE_URL || '/api/open/people',
  timeout: 15_000,
  withCredentials: true,
  xsrfCookieName: 'PEOPLE_XSRF',
  xsrfHeaderName: 'X-XSRF-TOKEN',
  headers: { 'Content-Type': 'application/json' },
})

async function unwrap<T>(request: Promise<{ data: Envelope<T> }>) {
  return (await request).data.data
}

let csrfReady = false
async function ensureCSRF() {
  if (!csrfReady) {
    await client.get('/auth/csrf')
    csrfReady = true
  }
}

async function mutation<T>(request: () => Promise<{ data: Envelope<T> }>) {
  await ensureCSRF()
  try {
    return await unwrap(request())
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.data?.code === 'INVALID_CSRF_TOKEN') {
      csrfReady = false
      await ensureCSRF()
      return unwrap(request())
    }
    throw error
  }
}

export const peopleApi = {
  login: (username: string, password: string) => mutation<Employee>(() => client.post('/auth/login', { username, password })),
  me: () => unwrap<Employee>(client.get('/auth/me')),
  logout: () => mutation<{ loggedOut: boolean }>(() => client.post('/auth/logout')),
  changePassword: (currentPassword: string, newPassword: string) =>
    mutation<Employee>(() => client.post('/auth/change-password', { currentPassword, newPassword })),
  updateMyProfile: (input: Pick<EmployeeInput, 'email' | 'phone' | 'emergencyContactName' | 'emergencyContactPhone' | 'emergencyContactRelation'>) => mutation<Employee>(() => client.put('/profile', input)),
  employees: (params: { q?: string; page: number; pageSize: number }) => unwrap<Page<Employee>>(client.get('/employees', { params })),
  createEmployee: (input: EmployeeInput) => mutation<Employee>(() => client.post('/employees', input)),
  updateEmployee: (id: string, input: EmployeeInput) => mutation<Employee>(() => client.put(`/employees/${id}`, input)),
  deleteEmployee: (id: string) => mutation<{ deleted: boolean }>(() => client.delete(`/employees/${id}`)),
  resetEmployeePassword: (id: string) => mutation<{ reset: boolean }>(() => client.post(`/employees/${id}/reset-password`)),
  setEmployeeEnabled: (id: string, enabled: boolean) => mutation<Employee>(() => client.put(`/employees/${id}/enabled`, { enabled })),
  employmentEvents: (id: string) => unwrap<EmploymentEvent[]>(client.get(`/employees/${id}/events`)),
  departments: (params?: { q?: string }) => unwrap<Department[]>(client.get('/departments', { params })),
  createDepartment: (input: DepartmentInput) => mutation<Department>(() => client.post('/departments', input)),
  updateDepartment: (id: string, input: DepartmentInput) => mutation<Department>(() => client.put(`/departments/${id}`, input)),
  deleteDepartment: (id: string) => mutation<{ deleted: boolean }>(() => client.delete(`/departments/${id}`)),
  dashboard: () => unwrap<HRDashboard>(client.get('/hr/dashboard')),
  approvalTypes: () => unwrap<ApprovalTypeDefinition[]>(client.get('/approval-types')),
  approvals: (params?: { scope?: 'mine' | 'pending' | 'all'; type?: ApprovalType | ''; status?: string }) => unwrap<ApprovalRequest[]>(client.get('/approvals', { params })),
  approval: (id: string) => unwrap<ApprovalRequest>(client.get(`/approvals/${id}`)),
  createApproval: (input: { type: ApprovalType; data: Record<string, string> }) => mutation<ApprovalRequest>(() => client.post('/approvals', input)),
  reviewApproval: (id: string, input: { approved: boolean; comment: string }) => mutation<ApprovalRequest>(() => client.post(`/approvals/${id}/review`, input)),
  cancelApproval: (id: string) => mutation<{ cancelled: boolean }>(() => client.post(`/approvals/${id}/cancel`)),
  leaveBalance: (year?: number) => unwrap<LeaveBalance>(client.get('/leave/balance', { params: { year } })),
  leaveCalendar: (month?: string) => unwrap<LeaveRecord[]>(client.get('/leave/calendar', { params: { month } })),
  contracts: (employeeId?: string) => unwrap<EmployeeContract[]>(client.get('/contracts', { params: { employeeId } })),
  createContract: (employeeId: string, input: ContractInput) => mutation<EmployeeContract>(() => client.post(`/employees/${employeeId}/contracts`, input)),
  updateContract: (id: string, input: ContractInput) => mutation<EmployeeContract>(() => client.put(`/contracts/${id}`, input)),
  deleteContract: (id: string) => mutation<{ deleted: boolean }>(() => client.delete(`/contracts/${id}`)),
  goals: (cycle?: string) => unwrap<PerformanceGoal[]>(client.get('/performance-goals', { params: { cycle } })),
  createGoal: (input: GoalInput) => mutation<PerformanceGoal>(() => client.post('/performance-goals', input)),
  updateGoal: (id: string, input: GoalInput) => mutation<PerformanceGoal>(() => client.put(`/performance-goals/${id}`, input)),
  departures: () => unwrap<DepartureRequest[]>(client.get('/departures')),
  createDeparture: (input: { reason: string; lastWorkingDate: string }) => mutation<DepartureRequest>(() => client.post('/departures', input)),
  reviewDeparture: (id: string, stage: 'manager' | 'hr', input: { approved: boolean; comment: string }) =>
    mutation<DepartureRequest>(() => client.post(`/departures/${id}/${stage}-review`, input)),
  cancelDeparture: (id: string) => mutation<{ cancelled: boolean }>(() => client.post(`/departures/${id}/cancel`)),
  notifications: (unread = false) => unwrap<NotificationItem[]>(client.get('/notifications', { params: { unread } })),
  notificationSummary: () => unwrap<NotificationSummary>(client.get('/notifications/summary')),
  markNotificationRead: (id: string) => mutation<{ read: boolean }>(() => client.post(`/notifications/${id}/read`)),
  markAllNotificationsRead: () => mutation<{ read: boolean }>(() => client.post('/notifications/read-all')),
  authorize: (clientId: string, redirectUri: string, state: string, account?: { username: string; password: string }) =>
    mutation<{ redirectUrl: string }>(() => client.post('/oauth/authorize', { clientId, redirectUri, state, ...account })),
}

export function apiMessage(error: unknown, fallback = '请求失败') {
  if (axios.isAxiosError<Envelope<unknown>>(error)) return error.response?.data?.message || fallback
  return error instanceof Error ? error.message : fallback
}

export function isUnauthorized(error: unknown) {
  return axios.isAxiosError(error) && error.response?.status === 401
}
