import api from './http'

export const authApi = {
  login(payload) {
    return api.post('/auth/login', payload)
  },
  register(payload) {
    return api.post('/auth/register', payload)
  },
  me() {
    return api.get('/auth/me')
  },
  updateProfile(payload) {
    return api.put('/auth/me', payload)
  }
}

export const dashboardApi = {
  summary() {
    return api.get('/dashboard/summary')
  }
}

export const tendersApi = {
  list(params) {
    return api.get('/tenders', { params })
  },
  byId(id) {
    return api.get(`/tenders/${id}`)
  }
}

export const companiesApi = {
  list(params) {
    return api.get('/companies', { params })
  },
  byId(id) {
    return api.get(`/companies/${id}`)
  },
  create(payload) {
    return api.post('/companies', payload)
  },
  update(id, payload) {
    return api.put(`/companies/${id}`, payload)
  },
  remove(id) {
    return api.delete(`/companies/${id}`)
  }
}

export const analysisApi = {
  create(payload) {
    return api.post('/analysis/requests', payload)
  },
  list(params) {
    return api.get('/analysis/requests', { params })
  },
  requestById(id) {
    return api.get(`/analysis/requests/${id}`)
  },
  resultById(id) {
    return api.get(`/analysis/results/${id}`)
  }
}

export const integrationApi = {
  listMessages(params) {
    return api.get('/integration/messages', { params })
  }
}
