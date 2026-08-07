export interface Employee {
  id: string
  employeeNo: string
  username: string
  displayName: string
  email: string
  phone: string
  departmentId: string
  department: string
  title: string
  role: 'admin' | 'employee'
  status: 'enabled' | 'disabled'
  mustChangePassword: boolean
  passwordChangedAt?: string | null
  lastLoginAt?: string | null
  createdAt: string
  updatedAt: string
}

export interface EmployeeInput {
  employeeNo: string
  username: string
  displayName: string
  email: string
  phone: string
  departmentId: string
  title: string
  role: Employee['role']
  status: Employee['status']
}

export interface Department {
  id: string
  code: string
  name: string
  description: string
  status: 'enabled' | 'disabled'
  employeeCount: number
  createdAt: string
  updatedAt: string
}

export interface DepartmentInput {
  code: string
  name: string
  description: string
  status: Department['status']
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}
