import axios from 'axios'

export const http = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

http.interceptors.response.use(
  (resp) => resp,
  (err) => {
    const msg = err?.response?.data?.error || err.message || '请求失败'
    return Promise.reject(new Error(msg))
  }
)