<script>
  import { onMount } from 'svelte'
  import Sidebar from './Sidebar.svelte'
  import ChatPane from './ChatPane.svelte'
  import Login from './Login.svelte'
  import { authed, checkAuth, loadChats, chats, openChat } from './stores.js'
  import { connectWS } from './ws.js'

  let booted = false

  onMount(async () => {
    await checkAuth()
    booted = true
  })

  // Cuando hay sesión, carga chats y abre el WS (una sola vez).
  let started = false
  $: if ($authed && !started) {
    started = true
    ;(async () => {
      await loadChats()
      if ($chats.length) await openChat($chats[0].phone)
      connectWS()
    })()
  }
  $: if ($authed === false) started = false
</script>

{#if !booted}
  <div class="boot"></div>
{:else if $authed}
  <div class="layout">
    <Sidebar />
    <ChatPane />
  </div>
{:else}
  <Login />
{/if}

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; }
  :global(body) { margin: 0; font-family: 'Segoe UI', system-ui, sans-serif; background: #0b141a; color: #e9edef; height: 100dvh; overflow: hidden; }
  .layout { display: grid; grid-template-columns: 380px 1fr; height: 100dvh; }
  .boot { height: 100dvh; background: #0b141a; }
</style>
