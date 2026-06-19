<script>
  import { tick } from 'svelte'
  import { active, messages, activeChat, contacts, hasMore, pushMessage, loadChats, loadOlder, renameContact } from './stores.js'
  import TemplateModal from './TemplateModal.svelte'

  let draft = ''
  let messagesEl
  let showEmoji = false
  let showTemplate = false
  let emojiEl
  let editingName = false
  let nameInput = ''
  let lightbox = null   // url de imagen ampliada

  // Las URLs de media de CM son privadas → pasan por el proxy del backend.
  function mediaSrc(url) {
    return url ? '/media?url=' + encodeURIComponent(url) : ''
  }
  function openLightbox(src) { lightbox = src }

  // Autoscroll solo cuando llega un mensaje NUEVO al final (no al prepender historial antiguo).
  let lastBottomId = null
  $: {
    const last = $messages[$messages.length - 1]
    if (last && last.id !== lastBottomId) {
      lastBottomId = last.id
      scrollBottom()
    }
  }

  async function scrollBottom() {
    await tick()
    if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight
  }

  // Carga más historial al subir cerca del tope, conservando la posición de scroll.
  let loadingOlder = false
  async function onScroll() {
    if (!messagesEl || loadingOlder || !$hasMore || messagesEl.scrollTop > 60) return
    loadingOlder = true
    const prevH = messagesEl.scrollHeight
    const added = await loadOlder()
    await tick()
    if (added && messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight - prevH
    loadingOlder = false
  }

  // Tick de estado para mensajes salientes.
  function tickIcon(status) {
    if (status === 'failed') return '⚠'
    if (status === 'sent') return '✓'
    return '✓✓' // delivered | read
  }

  // ── Envío ────────────────────────────────────────────────────────────────
  async function send() {
    const text = draft.trim(), to = $active
    if (!text || !to) return
    draft = ''
    showEmoji = false
    const r = await fetch('/send', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ to, text }),
    })
    if (r.ok) { pushMessage(await r.json()); loadChats() }
  }

  async function onSendTemplate(e) {
    const { template, params, to, body } = e.detail
    showTemplate = false
    const r = await fetch('/send-template', {
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ to, namespace:template.namespace, name:template.name, lang:template.language||'es', body, params }),
    })
    if (r.ok) { pushMessage(await r.json()); loadChats() }
  }

  function handleKey(e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() }
  }

  // ── Emoji ────────────────────────────────────────────────────────────────
  async function toggleEmoji() {
    showEmoji = !showEmoji
    if (showEmoji && !customElements.get('emoji-picker')) {
      await import('emoji-picker-element')
    }
    if (showEmoji) {
      await tick()
      emojiEl?.addEventListener('emoji-click', e => { draft += e.detail.unicode }, { once:false })
    }
  }

  function outsideClick(e) {
    if (showEmoji && !e.target.closest('.emoji-anchor') && !e.target.closest('.emoji-btn')) {
      showEmoji = false
    }
  }

  // ── Renombrar contacto ───────────────────────────────────────────────────
  function startEdit() {
    nameInput = $contacts[$active] || ''
    editingName = true
    tick().then(() => document.getElementById('name-input')?.focus())
  }

  async function saveName() {
    editingName = false
    if (nameInput.trim() !== ($contacts[$active] || '')) {
      await renameContact($active, nameInput.trim())
    }
  }

  function nameKey(e) {
    if (e.key === 'Enter') saveName()
    if (e.key === 'Escape') editingName = false
  }

  // ── Formateo ─────────────────────────────────────────────────────────────
  function fmtTime(ts) {
    return ts ? new Date(ts).toLocaleTimeString('es', { hour:'2-digit', minute:'2-digit' }) : ''
  }

  function fmtDate(ts) {
    if (!ts) return ''
    const d = new Date(ts), now = new Date()
    const yesterday = new Date(now); yesterday.setDate(now.getDate()-1)
    if (d.toDateString() === now.toDateString()) return 'Hoy'
    if (d.toDateString() === yesterday.toDateString()) return 'Ayer'
    return d.toLocaleDateString('es', { day:'numeric', month:'long', year:'numeric' })
  }

  $: grouped = buildGroups($messages)

  function buildGroups(msgs) {
    const out = []; let lastDate = null
    for (const m of msgs) {
      const d = m.ts ? new Date(m.ts).toDateString() : ''
      if (d !== lastDate) { out.push({ sep:true, label:fmtDate(m.ts) }); lastDate = d }
      out.push({ sep:false, m })
    }
    return out
  }

  function avatarColor(phone) {
    const colors = ['#00a884','#027eb5','#7b61ff','#e05c5c','#d97706','#059669']
    let h = 0; for (const c of phone||'') h = (h*31 + c.charCodeAt(0)) & 0xffff
    return colors[h % colors.length]
  }
</script>

<svelte:window on:click={outsideClick} />

{#if showTemplate}
  <TemplateModal to={$active} on:close={() => showTemplate=false} on:send={onSendTemplate} />
{/if}

{#if lightbox}
  <div class="lightbox" on:click={() => lightbox=null}>
    <img src={lightbox} alt="imagen" />
  </div>
{/if}

<div class="pane">
  {#if $active}
    <!-- Header -->
    <header>
      <div class="avatar" style="background:{avatarColor($active)}">
        {($contacts[$active]||$active).slice(0,2).toUpperCase()}
      </div>
      <div class="contact-info">
        {#if editingName}
          <input
            id="name-input"
            class="name-edit"
            bind:value={nameInput}
            on:blur={saveName}
            on:keydown={nameKey}
            placeholder="Nombre del contacto"
          />
        {:else}
          <button class="name-btn" on:click={startEdit} title="Editar nombre">
            <strong>{$contacts[$active] || $active}</strong>
            {#if $contacts[$active]}<span class="phone-sub">{$active}</span>{/if}
          </button>
        {/if}
      </div>
      <div class="hdr-actions">
        <button class="icon-btn" on:click={() => showTemplate=true} title="Enviar plantilla">
          <svg viewBox="0 0 24 24" width="20" fill="currentColor"><path d="M14 2H6a2 2 0 0 0-2 2v16c0 1.1.89 2 2 2h12a2 2 0 0 0 2-2V8l-6-6zm-1 1.5L18.5 9H13V3.5zM6 20V4h5v7h7v9H6z"/></svg>
        </button>
      </div>
    </header>

    <!-- Mensajes -->
    <div class="messages" bind:this={messagesEl} on:scroll={onScroll}>
      {#if loadingOlder}<div class="loading-older">Cargando…</div>{/if}
      {#each grouped as item}
        {#if item.sep}
          <div class="date-sep"><span>{item.label}</span></div>
        {:else}
          {@const m = item.m}
          <div class="bwrap {m.direction}">
            <div class="bubble {m.direction}" class:failed={m.status==='failed'} class:media-bubble={m.media_type==='image'||m.media_type==='sticker'}>

              <!-- Sticker -->
              {#if m.media_type === 'sticker'}
                <img class="sticker" src={mediaSrc(m.media_url)} alt="sticker" loading="lazy" />

              <!-- Imagen -->
              {:else if m.media_type === 'image'}
                <img class="img-msg" src={mediaSrc(m.media_url)} alt="imagen" loading="lazy"
                  on:click={() => openLightbox(mediaSrc(m.media_url))} />
                {#if m.text}<p class="caption">{m.text}</p>{/if}

              <!-- Vídeo -->
              {:else if m.media_type === 'video'}
                <video class="video-msg" src={mediaSrc(m.media_url)} controls preload="metadata"></video>
                {#if m.text}<p class="caption">{m.text}</p>{/if}

              <!-- Audio -->
              {:else if m.media_type === 'audio'}
                <audio class="audio-msg" src={mediaSrc(m.media_url)} controls></audio>

              <!-- Documento -->
              {:else if m.media_type === 'document'}
                <a class="doc-msg" href={mediaSrc(m.media_url)} target="_blank" rel="noopener">
                  <svg viewBox="0 0 24 24" width="24" fill="currentColor"><path d="M14 2H6a2 2 0 0 0-2 2v16c0 1.1.9 2 2 2h12a2 2 0 0 0 2-2V8l-6-6zm-1 1.5L18.5 9H13V3.5zM12 18H8v-2h4v2zm4-4H8v-2h8v2zm0-4H8v-2h8v2z"/></svg>
                  <span>Descargar documento</span>
                </a>

              <!-- Plantilla: muestra el contenido real con un distintivo -->
              {:else if m.media_type === 'template'}
                <div class="tpl-content">
                  <span class="tpl-badge">📋 Plantilla</span>
                  <p>{m.text}</p>
                </div>

              <!-- Texto normal -->
              {:else}
                <p>{m.text}</p>
              {/if}

              <div class="meta">
                <time>{fmtTime(m.ts)}</time>
                {#if m.direction === 'out'}
                  <span class="tick" class:fail={m.status==='failed'} class:read={m.status==='read'} title={m.status}>
                    {tickIcon(m.status)}
                  </span>
                {/if}
              </div>
            </div>
          </div>
        {/if}
      {/each}
      {#if !$messages.length}
        <div class="no-msgs">Sin mensajes aún</div>
      {/if}
    </div>

    <!-- Input -->
    <div class="input-area">
      {#if showEmoji}
        <div class="emoji-anchor">
          <emoji-picker bind:this={emojiEl}></emoji-picker>
        </div>
      {/if}
      <div class="toolbar">
        <button class="icon-btn emoji-btn" on:click|stopPropagation={toggleEmoji} title="Emoji">
          <svg viewBox="0 0 24 24" width="22" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 14.5c-1.38 0-2.5-1.12-2.5-2.5h5c0 1.38-1.12 2.5-2.5 2.5zm5.5-4.5H8.5c-.28 0-.5-.22-.5-.5s.22-.5.5-.5h7c.28 0 .5.22.5.5s-.22.5-.5.5zm-.5-3c-.55 0-1-.45-1-1s.45-1 1-1 1 .45 1 1-.45 1-1 1zm-4 0c-.55 0-1-.45-1-1s.45-1 1-1 1 .45 1 1-.45 1-1 1z"/></svg>
        </button>
        <textarea
          rows="1"
          placeholder="Escribe un mensaje"
          bind:value={draft}
          on:keydown={handleKey}
        ></textarea>
        <button class="icon-btn send-btn" on:click={send} title="Enviar">
          <svg viewBox="0 0 24 24" width="22" fill="currentColor"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
        </button>
      </div>
    </div>

  {:else}
    <div class="empty-state">
      <div>
        <svg viewBox="0 0 80 80" width="80" fill="none"><circle cx="40" cy="40" r="38" stroke="#2a3942" stroke-width="2"/><path d="M24 30h32M24 40h22M24 50h28" stroke="#00a884" stroke-width="2.5" stroke-linecap="round"/></svg>
        <h2>VMVChat</h2>
        <p>Selecciona un chat o escribe un número<br>en el panel izquierdo para empezar.</p>
      </div>
    </div>
  {/if}
</div>

<style>
  .pane { display:flex; flex-direction:column; height:100%; background:#0b141a; overflow:hidden; }

  /* Header */
  header { display:flex; align-items:center; gap:12px; padding:10px 16px; background:#202c33; border-bottom:1px solid #111b21; flex-shrink:0; min-height:60px; }
  .avatar { width:40px; height:40px; border-radius:50%; display:grid; place-items:center; font-size:13px; font-weight:700; color:#fff; flex-shrink:0; }
  .contact-info { flex:1; min-width:0; }
  .name-btn { background:none; border:none; cursor:pointer; text-align:left; padding:0; width:100%; }
  .name-btn strong { display:block; font-size:15px; color:#e9edef; }
  .name-btn:hover strong { color:#00a884; }
  .phone-sub { display:block; font-size:12px; color:#8696a0; }
  .name-edit { background:#2a3942; border:none; border-radius:6px; padding:4px 8px; color:#e9edef; font-size:15px; width:100%; outline:none; font-weight:600; }
  .name-edit:focus { box-shadow:0 0 0 2px #00a884; }
  .hdr-actions { display:flex; gap:4px; }
  .icon-btn { background:none; border:none; cursor:pointer; color:#8696a0; padding:8px; border-radius:50%; display:grid; place-items:center; }
  .icon-btn:hover { color:#e9edef; background:#2a3942; }

  /* Messages */
  .messages { flex:1; overflow-y:auto; padding:12px 8%; display:flex; flex-direction:column; gap:1px; scroll-behavior:smooth; }

  .date-sep { display:flex; justify-content:center; margin:10px 0; }
  .date-sep span { background:#182229; color:#8696a0; font-size:12px; padding:4px 12px; border-radius:8px; }

  .bwrap { display:flex; margin:1px 0; }
  .bwrap.in { justify-content:flex-start; }
  .bwrap.out { justify-content:flex-end; }

  .bubble { max-width:65%; padding:7px 12px 4px; border-radius:8px; word-break:break-word; position:relative; }
  .bubble.in { background:#202c33; border-radius:0 8px 8px 8px; }
  .bubble.out { background:#005c4b; border-radius:8px 0 8px 8px; }
  .bubble.failed { background:#3d1a1a !important; }
  .bubble.media-bubble { padding:4px 4px 4px; }
  .bubble p { margin:0 0 4px; font-size:14.5px; line-height:1.4; color:#e9edef; white-space:pre-wrap; }

  /* Media */
  .sticker { width:160px; height:160px; object-fit:contain; background:none; display:block; }
  .img-msg { max-width:280px; max-height:280px; object-fit:cover; border-radius:6px; display:block; cursor:zoom-in; }
  .video-msg { max-width:280px; border-radius:6px; display:block; }
  .audio-msg { width:220px; }
  .caption { margin:4px 8px 2px !important; font-size:13px !important; color:#d1d7db !important; }
  .doc-msg { display:flex; align-items:center; gap:10px; padding:6px 8px; background:rgba(255,255,255,.07); border-radius:6px; color:#e9edef; text-decoration:none; }
  .doc-msg:hover { background:rgba(255,255,255,.12); }
  .doc-msg span { font-size:13px; }
  .tpl-content { display:flex; flex-direction:column; gap:4px; }
  .tpl-badge { font-size:11px; color:#8696a0; border-bottom:1px solid rgba(255,255,255,.1); padding-bottom:3px; }
  .tpl-content p { margin:0 !important; }

  .lightbox { position:fixed; inset:0; background:rgba(0,0,0,.85); display:grid; place-items:center; z-index:100; cursor:zoom-out; }
  .lightbox img { max-width:90vw; max-height:90vh; border-radius:6px; }

  .meta { display:flex; align-items:center; justify-content:flex-end; gap:4px; padding:0 2px; }
  time { font-size:11px; color:#8696a0; }
  .tick { font-size:12px; color:#8696a0; }     /* sent / delivered */
  .tick.read { color:#53bdeb; }                 /* leído (azul) */
  .tick.fail { color:#f15c6d; }
  .no-msgs { color:#8696a0; font-size:14px; margin:auto; }
  .loading-older { text-align:center; color:#8696a0; font-size:12px; padding:8px; }

  /* Input */
  .input-area { flex-shrink:0; background:#202c33; border-top:1px solid #111b21; position:relative; }
  .emoji-anchor { position:absolute; bottom:100%; left:0; z-index:20; }
  emoji-picker { --background:#233138; --border-color:#2a3942; --indicator-color:#00a884; --input-background-color:#2a3942; --input-font-color:#e9edef; --emoji-size:1.3rem; width:340px; height:360px; }
  .toolbar { display:flex; align-items:flex-end; gap:6px; padding:10px 16px; }
  textarea { flex:1; background:#2a3942; border:none; border-radius:20px; padding:10px 16px; color:#e9edef; font-size:14.5px; resize:none; outline:none; max-height:140px; font-family:inherit; line-height:1.4; }
  textarea::placeholder { color:#8696a0; }
  .send-btn { background:#00a884 !important; color:#fff !important; border-radius:50% !important; width:42px; height:42px; flex-shrink:0; }
  .send-btn:hover { background:#02b893 !important; }

  /* Empty state */
  .empty-state { flex:1; display:grid; place-items:center; background:#222e35; text-align:center; }
  .empty-state h2 { color:#e9edef; margin:16px 0 8px; }
  .empty-state p { color:#8696a0; font-size:14px; line-height:1.6; margin:0; }
</style>
