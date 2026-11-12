import { post, postPage } from './http'

export function updateProfile(body) {
  return post('/api/account/profile', body)
}

export function changePassword(body) {
  return post('/api/account/password', body)
}

export function createTeam(body) {
  return post('/api/team/create', body)
}

export function updateTeam(body) {
  return post('/api/team/update', body)
}

export function deleteTeam(id) {
  return post('/api/team/delete', { id })
}

export function searchTeams(body) {
  return postPage('/api/team/search', body)
}

export function getTeam(id) {
  return post('/api/team/one', { id })
}

export function addTeamMember(body) {
  return post('/api/team/member/add', body)
}

export function removeTeamMember(body) {
  return post('/api/team/member/remove', body)
}

export function leaveTeam(teamId) {
  return post('/api/team/leave', { team_id: teamId })
}
