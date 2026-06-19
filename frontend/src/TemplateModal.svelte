<script>
  import { createEventDispatcher } from 'svelte'
  const dispatch = createEventDispatcher()

  export let to = ''

  let templates = []
  let selected = null
  let params = []
  let showAdd = false
  let newTpl = { name: '', namespace: '', language: 'es', category: '', body: '' }
  let loading = true

  async function load() {
    loading = true
    const r = await fetch('/api/templates')
    templates = r.ok ? await r.json() : []
    loading = false
  }

  function selectTpl(t) {
    selected = t
    params = []
  }

  async function saveTpl() {
    if (!newTpl.name || !newTpl.namespace) return
    const r = await fetch('/api/templates', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newTpl),
    })
    if (r.ok) {
      newTpl = { name: '', namespace: '', language: 'es', category: '', body: '' }
      showAdd = false
      await load()
    }
  }

  async function deleteTpl(name) {
    await fetch('/api/templates/' + encodeURIComponent(name), { method: 'DELETE' })
    if (selected?.name === name) selected = null
    await load()
  }

  async function send() {
    if (!selected || !to) return
    dispatch('send', { template: selected, params: params.filter(Boolean), to, body: selected.body })
  }

  import { onMount } from 'svelte'
  onMount(load)
</script>

<div class="overlay" on:click|self={() => dispatch('close')}>
  <div class="modal">
    <div class="modal-header">
      {#if selected}
        <button class="back-btn" on:click={() => selected = null}>←</button>
        <h3>Enviar plantilla</h3>
      {:else}
        <h3>Plantillas</h3>
        <div class="header-actions">
          <button class="icon-btn" on:click={() => showAdd = !showAdd} title="Añadir">＋</button>
        </div>
      {/if}
    </div>

    {#if !selected}
      {#if showAdd}
        <div class="form">
          <label>Nombre (facebook_name)
            <input bind:value={newTpl.name} placeholder="prueba_envio_en_camino" />
          </label>
          <label>Namespace (facebook_namespace_id)
            <input bind:value={newTpl.namespace} placeholder="4d1c775b_71bd_..." />
          </label>
          <label>Idioma
            <input bind:value={newTpl.language} placeholder="es" />
          </label>
          <label>Categoría
            <input bind:value={newTpl.category} placeholder="Utility" />
          </label>
          <label>Contenido (lo que se mostrará en el chat)
            <textarea bind:value={newTpl.body} rows="3" placeholder="📦 Su envío está en camino. Usa {'{'}1{'}'} para parámetros."></textarea>
          </label>
          <div class="actions">
            <button class="secondary" on:click={() => showAdd = false}>Cancelar</button>
            <button class="primary" on:click={saveTpl}>Guardar</button>
          </div>
        </div>
      {:else if loading}
        <p class="hint">Cargando...</p>
      {:else if !templates.length}
        <p class="hint">Sin plantillas. Pulsa ＋ para añadir una.</p>
      {:else}
        <ul class="tpl-list">
          {#each templates as t}
            <li>
              <div class="tpl-info" on:click={() => selectTpl(t)}>
                <strong>{t.name}</strong>
                <span>{t.language} · {t.category}</span>
              </div>
              <button class="del" on:click|stopPropagation={() => deleteTpl(t.name)}>✕</button>
            </li>
          {/each}
        </ul>
      {/if}
    {:else}
      <div class="send-form">
        <p class="tpl-title"><strong>{selected.name}</strong> <span>({selected.language})</span></p>
        {#if selected.body}
          <div class="preview">{selected.body}</div>
        {/if}
        <p class="hint">Añade valores para los parámetros {'{'}1{'}'}, {'{'}2{'}'}… si los tiene.</p>
        {#each params as _, i}
          <label>Parámetro {i + 1}
            <input bind:value={params[i]} placeholder="Valor" />
          </label>
        {/each}
        <button class="add-param" on:click={() => params = [...params, '']}>+ Parámetro</button>
        <div class="actions">
          <button class="secondary" on:click={() => selected = null}>← Atrás</button>
          <button class="primary" on:click={send}>Enviar</button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: grid; place-items: center; z-index: 50; }
  .modal { background: #202c33; border-radius: 12px; padding: 24px; width: 440px; max-width: 95vw; max-height: 80vh; overflow-y: auto; color: #e9edef; }
  .modal-header { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; }
  .modal-header h3 { margin: 0; font-size: 17px; flex: 1; }
  .back-btn { background: none; border: none; color: #e9edef; font-size: 20px; cursor: pointer; padding: 0; line-height: 1; }
  .header-actions { display: flex; gap: 6px; }
  .icon-btn { background: #00a884; color: #fff; border: none; border-radius: 50%; width: 30px; height: 30px; font-size: 18px; cursor: pointer; display: grid; place-items: center; }

  .tpl-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
  .tpl-list li { display: flex; align-items: center; gap: 8px; background: #2a3942; border-radius: 10px; padding: 12px 14px; }
  .tpl-list li:hover { background: #324a56; }
  .tpl-info { flex: 1; cursor: pointer; }
  .tpl-info strong { display: block; margin-bottom: 2px; font-size: 14px; }
  .tpl-info span { font-size: 12px; color: #8696a0; }
  .del { background: none; border: none; color: #8696a0; cursor: pointer; font-size: 13px; padding: 4px 6px; border-radius: 4px; }
  .del:hover { color: #f15c6d; background: rgba(241,92,109,.1); }

  .form, .send-form { display: flex; flex-direction: column; gap: 12px; }
  label { display: flex; flex-direction: column; gap: 5px; font-size: 13px; color: #8696a0; }
  input, textarea { padding: 9px 12px; background: #2a3942; border: none; border-radius: 8px; color: #e9edef; font-size: 14px; outline: none; font-family: inherit; resize: vertical; }
  input:focus, textarea:focus { box-shadow: 0 0 0 2px #00a884; }
  .preview { background: #005c4b; border-radius: 8px; padding: 10px 12px; font-size: 14px; white-space: pre-wrap; line-height: 1.4; }
  .tpl-title { margin: 0 0 4px; }
  .tpl-title span { color: #8696a0; font-size: 13px; font-weight: normal; }
  .add-param { align-self: flex-start; background: #2a3942; border: none; border-radius: 8px; color: #e9edef; padding: 7px 14px; cursor: pointer; font-size: 13px; }
  .add-param:hover { background: #324a56; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 6px; }
  .actions button { padding: 9px 20px; border: none; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 500; }
  .primary { background: #00a884; color: #fff; }
  .primary:hover { background: #02b893; }
  .secondary { background: #2a3942; color: #e9edef; }
  .secondary:hover { background: #324a56; }
  .hint { color: #8696a0; font-size: 13px; margin: 0; }
</style>
