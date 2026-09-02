import { useEffect, useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router'
import { Cloud, HardDrive, LogOut, Trash2, User as UserIcon } from 'lucide-react'
import { formatSize, getQuota } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const navItems = [
  { to: '/', label: '我的网盘', icon: HardDrive, end: true },
  { to: '/trash', label: '回收站', icon: Trash2 },
  { to: '/profile', label: '个人中心', icon: UserIcon },
]

export default function AppLayout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [quota, setQuota] = useState({ quota: 1, used: 0 })

  const refreshQuota = () => {
    getQuota().then(setQuota).catch(() => {})
  }

  useEffect(() => {
    refreshQuota()
  }, [])

  const percent = Math.min(100, Math.round((quota.used / quota.quota) * 100))

  return (
    <div className="min-h-screen flex bg-slate-50">
      <aside className="w-60 shrink-0 border-r bg-white flex flex-col">
        <div className="flex items-center gap-2 px-4 h-16 border-b">
          <Cloud className="h-6 w-6 text-sky-600" />
          <span className="font-bold text-slate-800">Chirp CloudDrive</span>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {navItems.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  isActive ? 'bg-sky-100 text-sky-700' : 'text-slate-600 hover:bg-slate-100',
                )
              }
            >
              <Icon className="h-4 w-4" />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="p-4 border-t space-y-3">
          <div>
            <div className="flex justify-between text-xs text-slate-500 mb-1">
              <span>存储空间</span>
              <span>
                {formatSize(quota.used)} / {formatSize(quota.quota)}
              </span>
            </div>
            <Progress value={percent} className="h-2" />
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-slate-600 truncate" title={user?.email}>
              {user?.name}
            </span>
            <Button
              variant="ghost"
              size="icon"
              title="退出登录"
              onClick={() => {
                logout()
                navigate('/login')
              }}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </aside>
      <main className="flex-1 p-6 overflow-auto">
        <Outlet context={{ refreshQuota }} />
      </main>
    </div>
  )
}
