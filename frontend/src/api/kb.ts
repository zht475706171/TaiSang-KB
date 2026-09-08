import { http } from './client'
import type { KnowledgeBase } from '@/types'

export const kbApi = {
  list: () => http.get<{ items: KnowledgeBase[] }>('/kb').then(r => r.data.items),
  get: (id: string) => http.get<KnowledgeBase>(`/kb/${id}`).then(r => r.data),
  create: (data: { name: string; description?: string; embedding_model?: string }) =>
    http.post<KnowledgeBase>('/kb', data).then(r => r.data),
  update: (id: string, data: Partial<KnowledgeBase>) =>
    http.put<KnowledgeBase>(`/kb/${id}`, data).then(r => r.data),
  delete: (id: string) => http.delete(`/kb/${id}`),
}