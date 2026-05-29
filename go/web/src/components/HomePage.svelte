<script lang="ts">
import type { ProjectData } from "../types";
import ProjectShell from "./ProjectShell.svelte";

export let data: ProjectData;

$: rows = data.boardRows || [];
$: total = data.boardTotal || 0;
</script>

<ProjectShell active="home" heading={data.heading || "LSX"}>
  <section class="home-layout">
    <article class="leaderboard-card">
      <header class="card-header">
        <figure class="brand-lockup">
          <img src="/project/asset/lt2_icon_lsx.avif" alt="">
          <figcaption>
            <p class="eyebrow">Market Open</p>
            <h2 class="compact-heading">Lemonade Stock Exchange</h2>
          </figcaption>
        </figure>
      </header>

      <p class="leaderboard-summary">
        {#if total}
          {total}
          LSX submission{total === 1 ? "" : "s"}
          currently ranked.
        {:else}
          No LSX submissions have been received yet.
        {/if}
      </p>

      <section class="board-table" aria-label="Leaderboard preview">
        <header class="board-row head">
          <span>Rank</span>
          <span>Company</span>
          <span>Market Cap</span>
        </header>
        {#if rows.length}
          {#each rows as row}
            <p class="board-row body">
              <b>{row.Rank}</b>
              <span>{row.Company}</span>
              <span>${row.MarketCap}</span>
            </p>
          {/each}
        {:else}
          <p class="board-row body">
            <b>-</b>
            <span>No submissions</span>
            <span>$0.00</span>
          </p>
        {/if}
      </section>
    </article>

    <aside class="side-stack">
      <a class="game-button" href="/board">
        <span>Open LSX Board</span>
        <img src="/project/asset/lt2_icon_lsx.avif" alt="">
      </a>

      <section class="admin-card sort-card">
        <h2>Sort Board</h2>
        <nav class="sort-grid" aria-label="Leaderboard sort links">
          <a class="sort-link" href="/board?sort=0">Rank</a>
          <a class="sort-link" href="/board?sort=1">Company</a>
          <a class="sort-link" href="/board?sort=3">Lifespan</a>
          <a class="sort-link" href="/board?sort=4">Market Cap</a>
        </nav>
      </section>
    </aside>
  </section>

  <footer class="credits-bar">
    <h2 class="credit-title">
      <img class="h-14 w-14" src="/project/asset/lt2_asset_credits.avif" alt="">
      <span>Credits</span>
    </h2>
    <nav class="credit-list" aria-label="Project credits">
      <a class="credit-link" href="https://github.com/FlameFlag">
        <img src="/project/asset/flameflag_lemon.png" alt="">
        <span><b>Revival</b> FlameFlag</span>
      </a>
      <a class="credit-link" href="https://github.com/TiZmSpectrum">
        <img src="/project/asset/timz_lemon.png" alt="">
        <span><b>Idea & Domain</b> TiZmSpectrum</span>
      </a>
    </nav>
    <a class="source-link" href="https://github.com/FlameFlag/lsx-server">
      <img src="/project/asset/lt2_asset_github.avif" alt="">
      Source
    </a>
  </footer>
</ProjectShell>
