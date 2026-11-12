import { createRouter, createWebHistory } from 'vue-router'
import { useSessionStore } from '../stores/session'
import LibraryView from '../views/LibraryView.vue'
import EditorView from '../views/EditorView.vue'
import LoginView from '../views/LoginView.vue'
import RegisterView from '../views/RegisterView.vue'
import AccountView from '../views/AccountView.vue'
import TeamView from '../views/TeamView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { public: true } },
    { path: '/', name: 'library', component: LibraryView },
    { path: '/account', name: 'account', component: AccountView },
    { path: '/team/:id', name: 'team', component: TeamView },
    { path: '/doc/:id', name: 'editor', component: EditorView },
  ],
})

router.beforeEach(async (to) => {
  const session = useSessionStore()
  if (!session.loaded) await session.refresh()
  if (to.meta.public) {
    if (session.account) return { name: 'library' }
    return true
  }
  if (!session.account) return { name: 'login', query: { next: to.fullPath } }
  return true
})

export default router
