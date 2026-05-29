<script lang="ts">
import type { AdminLoginData } from "../types";

export let data: AdminLoginData;
</script>

<main class="login-screen">
  <span class="screen-map" aria-hidden="true"></span>
  <span class="screen-wash" aria-hidden="true"></span>

  <section class="login-panel">
    <header class="login-brand-header">
      <img src="/project/asset/lt2_logo_text_only.avif" alt="Lemonade Tycoon 2 New York Edition">
    </header>
    <form class="login-card" method="post" action={`${data.AdminPath}/login`}>
      <header class="login-lockup">
        <img src="/project/asset/lt2_icon_lsx.avif" alt="">
        <section>
          <h1 class="font-display text-3xl">Lemonade Stock Exchange</h1>
          <p class="login-subtitle">Protected admin console</p>
        </section>
      </header>

      {#if data.Error}
        <p class="status-message error">{data.Error}</p>
      {/if}

      <fieldset class="form-grid">
        <label class="field-label">
          <span>Username</span>
          <input class="admin-input" name="username" autocomplete="username" value={data.User || ""}>
        </label>
        <label class="field-label">
          <span>Password</span>
          <input class="admin-input" name="password" type="password" autocomplete="current-password">
        </label>
      </fieldset>

      <button class="game-button login-submit" type="submit">
        <span>Log in</span>
        <img src="/project/asset/lt2_icon_play.avif" alt="">
      </button>
    </form>
  </section>
</main>
