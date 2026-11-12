import { post, postPage } from './http'

export function createDocument(body) {
  return post('/api/document/create', body)
}

export function updateDocument(body) {
  return post('/api/document/update', body)
}

export function getDocument(id) {
  return post('/api/document/one', { id })
}

export function searchDocuments(body) {
  return postPage('/api/document/search', body)
}

export function deleteDocument(id) {
  return post('/api/document/delete', { id })
}

export function importDocument(body) {
  return post('/api/document/import', body)
}

export function exportDocument(id) {
  return post('/api/document/export', { id })
}

export function saveDocument(body) {
  return post('/api/document/save', body)
}

export function searchVersions(body) {
  return postPage('/api/document/version/search', body)
}

export function getVersion(body) {
  return post('/api/document/version/one', body)
}

export function restoreVersion(body) {
  return post('/api/document/version/restore', body)
}

export function moveDocument(body) {
  return post('/api/document/move', body)
}

export function createFolder(body) {
  return post('/api/document/folder/create', body)
}

export function updateFolder(body) {
  return post('/api/document/folder/update', body)
}

export function deleteFolder(id) {
  return post('/api/document/folder/delete', { id })
}

export function getFolder(id) {
  return post('/api/document/folder/one', { id })
}

export function searchFolders(body) {
  return postPage('/api/document/folder/search', body)
}
