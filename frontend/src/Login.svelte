<script>
  import { login } from './stores.js'

  let user = '', password = '', error = '', busy = false

  async function submit() {
    if (!user || !password) return
    busy = true; error = ''
    const ok = await login(user, password)
    busy = false
    if (!ok) { error = 'Usuario o contraseña incorrectos'; password = '' }
  }
</script>

<div class="screen">
  <form class="card" on:submit|preventDefault={submit}>
    <div class="logo">
      <svg viewBox="0 0 100 100" width="56"><rect width="100" height="100" rx="22" fill="#00a884"/><path d="M50 22c-15 0-27 11-27 24 0 5 2 9 5 13l-3 11 12-3c4 2 8 3 13 3 15 0 27-11 27-24S65 22 50 22z" fill="white"/></svg>
    </div>
    <h1>VMVChat</h1>
    <p class="sub">Inicia sesión para continuar</p>

    <label>Usuario
      <input bind:value={user} autocomplete="username" autofocus />
    </label>
    <label>Contraseña
      <input type="password" bind:value={password} autocomplete="current-password" />
    </label>

    {#if error}<div class="error">{error}</div>{/if}

    <button type="submit" disabled={busy}>{busy ? 'Entrando…' : 'Entrar'}</button>
  </form>
</div>

<style>
  .screen { height:100dvh; display:grid; place-items:center; background:#0b141a; }
  .card { background:#202c33; padding:36px 32px; border-radius:14px; width:340px; max-width:92vw;
          display:flex; flex-direction:column; gap:14px; box-shadow:0 12px 40px rgba(0,0,0,.4); }
  .logo { display:flex; justify-content:center; }
  h1 { margin:6px 0 0; text-align:center; font-size:22px; color:#e9edef; }
  .sub { margin:0 0 8px; text-align:center; color:#8696a0; font-size:13px; }
  label { display:flex; flex-direction:column; gap:5px; font-size:13px; color:#8696a0; }
  input { padding:11px 13px; background:#2a3942; border:none; border-radius:8px; color:#e9edef; font-size:14px; outline:none; }
  input:focus { box-shadow:0 0 0 2px #00a884; }
  .error { background:rgba(241,92,109,.12); color:#f8a5af; font-size:13px; padding:8px 12px; border-radius:8px; }
  button { margin-top:6px; padding:11px; background:#00a884; color:#fff; border:none; border-radius:8px;
           font-size:15px; font-weight:600; cursor:pointer; }
  button:hover:not(:disabled) { background:#02b893; }
  button:disabled { opacity:.6; cursor:default; }
</style>
