<script lang="ts">
export let active: "home" | "findings" | "docs" | "activate";
export let heading: string;
</script>

<main class="project-screen" class:home-screen={active === "home"}>
  <span class="screen-map" aria-hidden="true"></span>
  <span class="screen-wash" aria-hidden="true"></span>

  <section class="project-shell">
    <a class="main-menu-link" href="/">Main Menu</a>
    <header class="brand-header">
      <a class="brand-mark" href="/">
        <img class="brand-logo" src="/project/asset/lt2_logo_text_only.avif" alt="Lemonade Tycoon 2 New York Edition">
      </a>
      <nav class="project-nav" aria-label="Main menu">
        <a class:active-tab={active === "home"} class="project-tab" href="/">
          <span>Play</span>
          <img src="/project/asset/lt2_icon_play.avif" alt="">
        </a>
        <a class="project-tab" href="/board">
          <span>LSX</span>
          <img src="/project/asset/lt2_icon_lsx.avif" alt="">
        </a>
        <a
          class:active-tab={active === "findings"}
          class="project-tab"
          href="/findings"
          aria-current={active === "findings" ? "page" : undefined}
        >
          <span>Findings</span>
          <img src="/project/asset/lt2_icon_findings.avif" alt="">
        </a>
        <a
          class:active-tab={active === "activate"}
          class="project-tab"
          href="/activate"
          aria-current={active === "activate" ? "page" : undefined}
        >
          <span>Activate</span>
          <img src="/project/asset/lt2_icon_play.avif" alt="">
        </a>
        <a
          class:active-tab={active === "docs"}
          class="project-tab"
          href="/docs"
          aria-current={active === "docs" ? "page" : undefined}
        >
          <span>Docs</span>
          <span class="help-badge"><img src="/project/asset/lt2_asset_github.avif" alt=""></span>
        </a>
      </nav>
    </header>

    <section class="project-panel">
      <h1 class="panel-title">{heading}</h1>
      <section class="panel-body">
        <slot />
      </section>
    </section>
  </section>
</main>
