export type EmployeeStatus = 'enabled' | 'disabled'
export type EmploymentType = 'full_time' | 'part_time' | 'contract' | 'intern'

export interface Employee {
  id: string
  employeeNo: number
  username: string
  displayName: string
  email: string
  phone: string
  departmentId: string
  department: string
  title: string
  employmentType: EmploymentType
  hireDate: string
  probationEndDate: string
  workLocation: string
  role: 'admin' | 'employee'
  status: EmployeeStatus
  permissions: string[]
  mustChangePassword: boolean
  passwordChangedAt?: string | null
  lastLoginAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface EmployeeInput {
  username: string
  displayName: string
  email: string
  phone: string
  departmentId: string
  title: string
  employmentType: EmploymentType
  hireDate: string
  probationEndDate: string
  workLocation: string
}

export interface Department {
  id: string
  parentId: string
  code: string
  name: string
  description: string
  leaderId: string
  leaderName: string
  status: EmployeeStatus
  employeeCount: number
  createdAt: string
  updatedAt: string
  children?: Department[]
}

export interface DepartmentInput {
  parentId: string
  code: string
  name: string
  description: string
  leaderId: string
  status: EmployeeStatus
}

export type DepartureStatus = 'pending_manager' | 'pending_hr' | 'approved' | 'rejected' | 'cancelled'

export interface DepartureRequest {
  id: string
  employeeId: string
  employeeName: string
  employeeNo: number
  departmentId: string
  departmentName: string
  departmentLeaderId: string
  reason: string
  lastWorkingDate: string
  status: DepartureStatus
  managerReviewerId: string
  managerReviewerName: string
  managerReviewComment: string
  managerReviewedAt?: string | null
  hrReviewerId: string
  hrReviewerName: string
  hrReviewComment: string
  hrReviewedAt?: string | null
  canManagerReview: boolean
  canHrReview: boolean
  canCancel: boolean
  createdAt: string
  updatedAt: string
}

export interface NotificationItem {
  id: string
  type: string
  title: string
  content: string
  resourceType: string
  resourceId: string
  readAt?: string | null
  createdAt: string
}

export interface NotificationSummary {
  unread: number
  pendingTasks: number
  total: number
}

export interface HRDashboard {
  totalEmployees: number
  enabledEmployees: number
  disabledEmployees: number
  departments: number
  pendingDepartures: number
  probationEmployees: number
  recentHires: number
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}
