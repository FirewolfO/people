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
  positionId: string
  title: string
  employmentType: EmploymentType
  hireDate: string
  probationEndDate: string
  workLocation: string
  emergencyContactName: string
  emergencyContactPhone: string
  emergencyContactRelation: string
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
  positionId: string
  employmentType: EmploymentType
  hireDate: string
  probationEndDate: string
  workLocation: string
  emergencyContactName: string
  emergencyContactPhone: string
  emergencyContactRelation: string
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

export interface Position {
  id: string
  code: string
  name: string
  description: string
  status: EmployeeStatus
  builtin: boolean
  departmentIds: string[]
  departmentNames: string[]
  employeeCount: number
  createdAt: string
  updatedAt: string
}

export interface PositionInput {
  code: string
  name: string
  description: string
  status: EmployeeStatus
  departmentIds: string[]
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
  pendingApprovals: number
  probationEmployees: number
  recentHires: number
  employeesOnLeave: number
  contractsExpiring: number
  activeGoals: number
  overdueGoals: number
  departmentDistribution: MetricBucket[]
  employmentTypeDistribution: MetricBucket[]
  approvalDistribution: MetricBucket[]
}

export interface MetricBucket { name: string; count: number }

export type ApprovalType = 'leave' | 'transfer' | 'departure'
export type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'
export type ApprovalStepStatus = 'waiting' | 'pending' | 'approved' | 'rejected' | 'skipped'

export interface ApprovalStep {
  id: number
  sequence: number
  name: string
  approverId: string
  permissionCode: string
  status: ApprovalStepStatus
  reviewerId: string
  reviewerName: string
  comment: string
  reviewedAt?: string | null
}

export interface ApprovalRequest {
  id: string
  type: ApprovalType
  title: string
  summary: string
  applicantId: string
  applicantName: string
  applicantNo: number
  departmentId: string
  departmentName: string
  data: Record<string, string | number>
  status: ApprovalStatus
  currentStep: number
  totalSteps: number
  currentStepName: string
  steps: ApprovalStep[]
  canReview: boolean
  canCancel: boolean
  submittedAt: string
  completedAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface ApprovalTypeDefinition {
  code: ApprovalType
  name: string
  description: string
  steps: string[]
}

export interface LeaveBalance {
  employeeId: string
  year: number
  annualEntitlement: number
  annualUsed: number
  annualPending: number
  annualRemaining: number
  sickUsed: number
  personalUsed: number
}

export interface LeaveRecord {
  id: string
  approvalId: string
  employeeId: string
  employeeName: string
  departmentId: string
  departmentName: string
  leaveType: string
  startDate: string
  endDate: string
  days: number
  reason: string
  status: ApprovalStatus
}

export interface EmploymentEvent {
  id: string
  employeeId: string
  type: 'hire' | 'transfer' | 'promotion' | 'departure' | 'enable' | 'disable'
  effectiveDate: string
  fromDepartmentId: string
  fromDepartment: string
  toDepartmentId: string
  toDepartment: string
  fromTitle: string
  toTitle: string
  note: string
  approvalId: string
  createdAt: string
}

export interface EmployeeContract {
  id: string
  employeeId: string
  employeeName: string
  type: 'fixed_term' | 'open_ended' | 'internship' | 'service'
  startDate: string
  endDate: string
  status: 'active' | 'ended' | 'terminated'
  documentName: string
  note: string
  createdAt: string
  updatedAt: string
}

export type ContractInput = Pick<EmployeeContract, 'type' | 'startDate' | 'endDate' | 'status' | 'documentName' | 'note'>

export interface PerformanceGoal {
  id: string
  employeeId: string
  employeeName: string
  departmentId: string
  cycle: string
  title: string
  description: string
  dueDate: string
  weight: number
  progress: number
  status: 'draft' | 'active' | 'completed' | 'cancelled'
  managerComment: string
  canEdit: boolean
  canReview: boolean
  createdAt: string
  updatedAt: string
}

export type GoalInput = Pick<PerformanceGoal, 'cycle' | 'title' | 'description' | 'dueDate' | 'weight' | 'progress' | 'status' | 'managerComment'>

export interface Page<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}
