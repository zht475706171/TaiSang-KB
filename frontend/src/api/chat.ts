import { fetchEventSource } from '@microsoft/fetch-event-source'
import type { Citation } from '@/types'

export interface ChatCallbacks {
  onDelta: (delta: string) => void
  onCitations: (citations: Citation[]) => void
  onError: (err: string) => void
  onDone: () => void
}

export const chatApi = {
  stream: async (kbId: string, question: string, sessionId: string | null, cb: ChatCallbacks) => {
    const ctrl = new AbortController()
    await fetchEventSource('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ kb_id: kbId, question, session_id: sessionId || '' }),
      signal: ctrl.signal,
      onmessage(ev) {
        if (ev.data === '[DONE]') { cb.onDone(); return }
        try {
          const payload = JSON.parse(ev.data)
          if (payload.error) { cb.onError(payload.error); return }
          if (payload.delta) { cb.onDelta(payload.delta); return }
          if (payload.citations) { cb.onCitations(payload.citations as Citation[]); return }
        } catch (e) {
          // ignore malformed
        }
      },
      onerror(err) {
        cb.onError(String(err))
        throw err
      },
    })
    return ctrl
  },
}