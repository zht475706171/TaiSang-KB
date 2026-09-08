export interface KnowledgeBase {
  id: string
  name: string
  description: string
  embedding_model: string
  created_at: string
  updated_at: string
}

export interface Document {
  id: string
  kb_id: string
  title: string
  source: string
  file_path: string
  mime_type: string
  status: 'pending' | 'parsing' | 'done' | 'failed'
  error_msg: string
  meta: Record<string, any>
  created_at: string
}

export interface Citation {
  chunk_id: string
  doc_id: string
  doc_title: string
  content_snippet: string
  score: number
}

export interface Setting {
  id: number
  api_base_url: string
  api_key: string
  chat_model: string
  embedding_model: string
  rerank_enabled: boolean
  top_k: number
}