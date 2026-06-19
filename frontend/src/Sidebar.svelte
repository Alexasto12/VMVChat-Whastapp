<script>
  import { chats, active, openChat, contacts, wsConnected, logout, searchMessages } from './stores.js'

  let search = ''
  let newPhone = ''
  let msgResults = []
  let searchToken = 0

  $: filtered = search.trim()
    ? $chats.filter(c => {
        const name = ($contacts[c.phone] || c.phone).toLowerCase()
        return name.includes(search.toLowerCase()) || c.last?.toLowerCase().includes(search.toLowerCase())
      })
    : $chats

  // Búsqueda en el contenido de los mensajes (backend), además del filtro de chats.
  $: runSearch(search)
  async function runSearch(q) {
    const my = ++searchToken
    if (q.trim().length < 2) { msgResults = []; return }
    const res = await searchMessages(q)
    if (my === searchToken) msgResults = res
  }

  function openResult(phone) { openChat(phone); search = ''; msgResults = [] }

  function formatTime(ts) {
    if (!ts) return ''
    const d = new Date(ts), now = new Date()
    return d.toDateString() === now.toDateString()
      ? d.toLocaleTimeString('es', { hour: '2-digit', minute: '2-digit' })
      : d.toLocaleDateString('es', { day: '2-digit', month: '2-digit' })
  }

  function previewText(c) {
    const map = { image:'📷 Foto', sticker:'🎉 Sticker', video:'🎥 Vídeo', audio:'🎤 Audio', document:'📎 Documento', template:'📋 Plantilla' }
    return map[c.last_media] || c.last || ''
  }

  function avatarInitials(phone) {
    const name = $contacts[phone]
    return name ? name.slice(0, 2).toUpperCase() : phone.slice(-2)
  }

  function avatarColor(phone) {
    const colors = ['#00a884','#027eb5','#7b61ff','#e05c5c','#d97706','#059669']
    let h = 0; for (const c of phone || '') h = (h * 31 + c.charCodeAt(0)) & 0xffff
    return colors[h % colors.length]
  }

  function highlight(text, q) {
    if (!text) return ''
    const i = text.toLowerCase().indexOf(q.toLowerCase())
    if (i < 0) return text.length > 60 ? text.slice(0, 60) + '…' : text
    const start = Math.max(0, i - 20)
    return (start > 0 ? '…' : '') + text.slice(start, i + q.length + 30) + '…'
  }

  function handleNewPhone(e) {
    if (e.key === 'Enter' && newPhone.trim()) { openChat(newPhone.trim()); newPhone = '' }
  }
</script>

<aside>
  <div class="sidebar-header">
    <span class="title">
      VMVChat
      <span class="dot" class:on={$wsConnected} title={$wsConnected ? 'Conectado' : 'Reconectando…'}></span>
    </span>
    <button class="logout" on:click={logout} title="Cerrar sesión">
      <svg viewBox="0 0 24 24" width="18" fill="currentColor"><path d="M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.58L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z"/></svg>
    </button>
  </div>

  <div class="search-wrap">
    <div class="search-box">
      <svg viewBox="0 0 24 24" width="15" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
      <input bind:value={search} placeholder="Buscar chats o mensajes" />
      {#if search}<button class="clear" on:click={() => { search=''; msgResults=[] }}>✕</button>{/if}
    </div>
  </div>

  <div class="new-chat">
    <input bind:value={newPhone} placeholder="Nuevo número (+34...)" on:keydown={handleNewPhone} />
    <button on:click={() => { if (newPhone.trim()) { openChat(newPhone.trim()); newPhone = '' } }} title="Abrir chat">
      <svg viewBox="0 0 24 24" width="16" fill="currentColor"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2z"/></svg>
    </button>
  </div>

  <div class="scroll">
    <ul class="chat-list">
      {#each filtered as c (c.phone)}
        {@const name = $contacts[c.phone] || c.phone}
        <li class:active={c.phone === $active} on:click={() => openChat(c.phone)} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && openChat(c.phone)}>
          <div class="avatar" style="background:{avatarColor(c.phone)}">{avatarInitials(c.phone)}</div>
          <div class="info">
            <div class="row">
              <strong class="name">{name}</strong>
              <time>{formatTime(c.ts)}</time>
            </div>
            <div class="row">
              <span class="preview">{previewText(c)}</span>
              {#if c.unread > 0}<span class="badge">{c.unread > 99 ? '99+' : c.unread}</span>{/if}
            </div>
          </div>
        </li>
      {/each}
      {#if !filtered.length && !msgResults.length}
        <li class="empty">Sin conversaciones</li>
      {/if}
    </ul>

    {#if msgResults.length}
      <div class="section">Mensajes</div>
      <ul class="chat-list">
        {#each msgResults as m (m.id)}
          {@const name = $contacts[m.chat] || m.chat}
          <li on:click={() => openResult(m.chat)} role="button" tabindex="0" on:keydown={e => e.key==='Enter' && openResult(m.chat)}>
            <div class="avatar" style="background:{avatarColor(m.chat)}">{avatarInitials(m.chat)}</div>
            <div class="info">
              <div class="row"><strong class="name">{name}</strong><time>{formatTime(m.ts)}</time></div>
              <span class="preview">{highlight(m.text, search)}</span>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</aside>

<style>
  aside { display:flex; flex-direction:column; background:#111b21; border-right:1px solid #222e35; overflow:hidden; height:100%; }

  .sidebar-header { padding:12px 16px; background:#202c33; display:flex; align-items:center; justify-content:space-between; flex-shrink:0; }
  .title { font-weight:700; font-size:18px; color:#e9edef; display:flex; align-items:center; gap:8px; }
  .dot { width:8px; height:8px; border-radius:50%; background:#f15c6d; transition:background .3s; }
  .dot.on { background:#00d066; }
  .logout { background:none; border:none; color:#8696a0; cursor:pointer; padding:6px; border-radius:50%; display:grid; place-items:center; }
  .logout:hover { color:#e9edef; background:#2a3942; }

  .search-wrap { padding:8px 12px; flex-shrink:0; }
  .search-box { display:flex; align-items:center; gap:8px; background:#202c33; border-radius:8px; padding:8px 14px; }
  .search-box svg { color:#8696a0; flex-shrink:0; }
  .search-box input { background:none; border:none; outline:none; color:#e9edef; font-size:14px; width:100%; }
  .search-box input::placeholder { color:#8696a0; }
  .clear { background:none; border:none; color:#8696a0; cursor:pointer; font-size:13px; padding:0; }

  .new-chat { display:flex; gap:6px; padding:4px 12px 8px; flex-shrink:0; }
  .new-chat input { flex:1; background:#202c33; border:none; border-radius:8px; padding:8px 12px; color:#e9edef; font-size:13px; outline:none; }
  .new-chat input::placeholder { color:#8696a0; }
  .new-chat button { background:#00a884; border:none; border-radius:8px; width:36px; cursor:pointer; display:grid; place-items:center; color:#fff; flex-shrink:0; }
  .new-chat button:hover { background:#02b893; }

  .scroll { overflow-y:auto; flex:1; }
  .chat-list { list-style:none; margin:0; padding:0; }
  .section { padding:10px 16px 4px; font-size:12px; color:#00a884; font-weight:600; text-transform:uppercase; letter-spacing:.5px; }
  .chat-list li { display:flex; align-items:center; gap:12px; padding:10px 16px; cursor:pointer; border-bottom:1px solid #1f2c34; transition:background .1s; }
  .chat-list li:hover { background:#182229; }
  .chat-list li.active { background:#2a3942; }
  .chat-list li:focus { outline:none; background:#182229; }

  .avatar { width:46px; height:46px; border-radius:50%; display:grid; place-items:center; font-size:14px; font-weight:700; color:#fff; flex-shrink:0; }

  .info { flex:1; min-width:0; }
  .row { display:flex; justify-content:space-between; align-items:center; gap:8px; }
  .row:first-child { align-items:baseline; margin-bottom:2px; }
  .name { font-size:15px; color:#e9edef; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
  time { font-size:11px; color:#8696a0; white-space:nowrap; flex-shrink:0; }
  .preview { font-size:13px; color:#8696a0; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
  .badge { background:#00a884; color:#04150f; font-size:11px; font-weight:700; min-width:20px; height:20px; padding:0 6px; border-radius:10px; display:grid; place-items:center; flex-shrink:0; }

  .empty { color:#8696a0; font-size:13px; text-align:center; padding:24px; cursor:default; }
  .empty:hover { background:none; }
</style>
