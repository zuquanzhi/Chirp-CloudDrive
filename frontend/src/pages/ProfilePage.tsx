import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { formatSize, getQuota, updateMe } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export default function ProfilePage() {
  const { user, refreshUser } = useAuth()
  const [quota, setQuota] = useState({ quota: 1, used: 0 })
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState({
    name: '',
    school: '',
    student_id: '',
    birthdate: '',
    address: '',
    gender: '',
  })

  useEffect(() => {
    getQuota().then(setQuota).catch(() => {})
  }, [])

  useEffect(() => {
    if (user) {
      setForm({
        name: user.name ?? '',
        school: user.school ?? '',
        student_id: user.student_id ?? '',
        birthdate: user.birthdate ?? '',
        address: user.address ?? '',
        gender: user.gender ?? '',
      })
    }
  }, [user])

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      await updateMe(form)
      await refreshUser()
      toast.success('资料已保存')
    } catch {
      toast.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  const percent = Math.min(100, Math.round((quota.used / quota.quota) * 100))

  return (
    <div className="max-w-2xl space-y-6">
      <h1 className="text-xl font-bold text-slate-800">个人中心</h1>

      <Card>
        <CardHeader>
          <CardTitle>存储空间</CardTitle>
          <CardDescription>你的网盘配额使用情况</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex justify-between text-sm text-slate-600 mb-2">
            <span>已使用 {formatSize(quota.used)}</span>
            <span>共 {formatSize(quota.quota)}（{percent}%）</span>
          </div>
          <Progress value={percent} className="h-3" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>账号信息</CardTitle>
          <CardDescription>{user?.email}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSave} className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="name">昵称</Label>
              <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="school">学校</Label>
              <Input id="school" value={form.school} onChange={(e) => setForm({ ...form, school: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="student_id">学号</Label>
              <Input id="student_id" value={form.student_id} onChange={(e) => setForm({ ...form, student_id: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="birthdate">生日</Label>
              <Input id="birthdate" type="date" value={form.birthdate} onChange={(e) => setForm({ ...form, birthdate: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="address">地址</Label>
              <Input id="address" value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label>性别</Label>
              <Select value={form.gender} onValueChange={(v) => setForm({ ...form, gender: v })}>
                <SelectTrigger>
                  <SelectValue placeholder="请选择" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="MALE">男</SelectItem>
                  <SelectItem value="FEMALE">女</SelectItem>
                  <SelectItem value="OTHER">其他</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="sm:col-span-2">
              <Button type="submit" disabled={saving}>
                {saving ? '保存中…' : '保存修改'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
