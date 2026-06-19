import { get } from 'svelte/store'
import { active, wsConnected, pushMessage, updateStatus, loadChats } from './stores.js'

let ws

export function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws`)

  ws.onopen = () => wsConnected.set(true)

  ws.onmessage = async (e) => {
    let ev
    try { ev = JSON.parse(e.data) } catch { return }

    if (ev.type === 'status') {
      updateStatus(ev.id, ev.status)
      return
    }
    if (ev.type === 'message') {
      if (ev.chat === get(active)) pushMessage(ev)
      await loadChats()
    }
  }

  ws.onclose = () => {
    wsConnected.set(false)
    setTimeout(connectWS, 2000)
  }
  ws.onerror = () => ws.close()
}
