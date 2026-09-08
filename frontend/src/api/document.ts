import { http } from './client'
import type { Document } from '@/types'

export const documentApi = {
  list: (kbId: string) => http.get<{ items: Document[] }>(`/kb/${kbId}/documents`).then(r => r.data.items),
  get: (id: string) => http.get<Document>(`/documents/${id}`).then(r => r.data),
  delete: (id: string) => http.delete(`/documents/${id}`),
  reparse: (id: string) => http.post(`/documents/${id}/reparse`),
  upload: (kbId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return http.post<Document>(`/kb/${kbId}/documents`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then(r => r.data)
  },
}