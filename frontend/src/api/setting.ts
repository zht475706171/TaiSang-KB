import { http } from './client'
import type { Setting } from '@/types'

export const settingApi = {
  get: () => http.get<Setting>('/setting').then(r => r.data),
  update: (data: Partial<Setting>) => http.put('/setting', data).then(r => r.data),
}