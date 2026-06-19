import { writable, derived, get } from 'svelte/store'

export const authed      = writable(null)   // null=desconocido, true, false
export const wsConnected = writable(false)
export const chats       = writable([])
export const active      = writable(null)    // phone string
export const messages    = writable([])
export const contacts    = writable({})      // {phone: name}
export const hasMore     = writable(false)   // ¿quedan mensajes más antiguos?

const enc = encodeURIComponent
const PAGE = 30

export const activeChat = derived(
  [chats, active],
  ([$chats, $active]) => $chats.find(c => c.phone === $active) ?? null
)

export function displayName(phone) {
  return get(contacts)[phone] || phone
}

// ── Auth ────────────────────────────────────────────────────────────────────
export async function checkAuth() {
  const r = await fetch('/api/me')
  const j = r.ok ? await r.json() : { authed: false }
  authed.set(!!j.authed)
  return !!j.authed
}

export async function login(user, password) {
  const r = await fetch('/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user, password }),
  })
  if (r.ok) { authed.set(true); return true }
  return false
}

export async function logout() {
  await fetch('/logout', { method: 'POST' })
  authed.set(false)
  chats.set([]); messages.set([]); active.set(null)
}

// ── Chats ───────────────────────────────────────────────────────────────────
export async function loadChats() {
  const r = await fetch('/api/chats')
  if (r.status === 401) { authed.set(false); return }
  const list = r.ok ? await r.json() : []
  chats.set(list)
  contacts.update(c => {
    for (const chat of list) if (chat.name) c[chat.phone] = chat.name
    return c
  })
}

export async function openChat(phone) {
  if (get(active) === phone) return
  active.set(phone)
  messages.set([])
  hasMore.set(false)
  const r = await fetch(`/api/chats/${enc(phone)}?limit=${PAGE}`)
  const list = r.ok ? await r.json() : []
  messages.set(list)
  hasMore.set(list.length >= PAGE)
  markRead(phone)
}

// loadOlder antepone la página anterior; devuelve cuántos mensajes añadió.
export async function loadOlder() {
  const ms = get(messages), phone = get(active)
  if (!ms.length || !phone) return 0
  const r = await fetch(`/api/chats/${enc(phone)}?before=${ms[0].id}&limit=${PAGE}`)
  const older = r.ok ? await r.json() : []
  if (older.length < PAGE) hasMore.set(false)
  if (older.length) messages.set([...older, ...ms])
  return older.length
}

export function pushMessage(m) {
  messages.update(ms => ms.some(x => x.id === m.id) ? ms : [...ms, m])
}

export function updateStatus(id, status) {
  messages.update(ms => ms.map(m => m.id === id ? { ...m, status } : m))
}

export async function markRead(phone) {
  await fetch(`/api/chats/${enc(phone)}/read`, { method: 'POST' })
  chats.update(cs => cs.map(c => c.phone === phone ? { ...c, unread: 0 } : c))
}

export async function searchMessages(q) {
  if (q.trim().length < 2) return []
  const r = await fetch('/api/search?q=' + enc(q))
  return r.ok ? await r.json() : []
}

export async function renameContact(phone, name) {
  const r = await fetch(`/api/contacts/${enc(phone)}`, {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  if (r.ok) {
    contacts.update(c => ({ ...c, [phone]: name }))
    await loadChats()
  }
}
