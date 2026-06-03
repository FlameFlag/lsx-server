<script lang="ts">
import createClient from "openapi-fetch";
import { onMount } from "svelte";
import type { paths } from "../openapi-types";
import type { ProjectData } from "../types";
import ProjectShell from "./ProjectShell.svelte";

export let data: ProjectData;

const api = createClient<paths>({ baseUrl: "" });
const sampleResponse = `{
  "data": [
    { "rank": 1, "company": "Example Stand", "ceo": "Player", "market_cents": 1234500 }
  ],
  "pagination": { "page": 1, "page_size": 10, "total_items": 81, "total_pages": 9 },
  "filters": {},
  "sort": "market"
}`;

let sampleRows: number | null = null;
let sampleError = "";

onMount(() => {
  void loadSample();
});

async function loadSample() {
  const { data: leaderboard, error } = await api.GET("/api/v1/leaderboard", {
    params: { query: { page_size: 3 } }
  });

  if (error) {
    sampleError = problemTitle(error);
    return;
  }
  sampleRows = leaderboard.data.length;
}

function problemTitle(error: unknown) {
  return error && typeof error === "object" && "title" in error ? String(error.title) : "Sample request failed";
}
</script>

<ProjectShell active="docs" heading={data.heading || "Docs"}>
  <article class="content-card">
    <header class="card-header">
      <section>
        <p class="eyebrow">OpenAPI</p>
        <h2 class="docs-title">LSX Server API</h2>
        <p class="docs-copy">Live LSX leaderboard contract and health checks.</p>
      </section>
      <nav class="docs-actions" aria-label="API resources">
        <a class="sort-link" href="/openapi.yaml">OpenAPI YAML</a>
        <a class="sort-link" href="/api/v1/leaderboard">Sample JSON</a>
      </nav>
    </header>

    <dl class="summary-grid" aria-label="OpenAPI summary">
      <section class="docs-stat">
        <dt>Version</dt>
        <dd>1.0.0</dd>
      </section>
      <section class="docs-stat">
        <dt>Endpoints</dt>
        <dd>2</dd>
      </section>
      <section class="docs-stat">
        <dt>Typed sample</dt>
        <dd>{sampleRows ?? (sampleError || "...")}</dd>
      </section>
    </dl>

    <section class="api-reference" aria-label="OpenAPI reference">
      <article class="endpoint-card">
        <header class="endpoint-head">
          <span class="endpoint-method get">GET</span>
          <code>/api/v1/leaderboard</code>
        </header>
        <p>Returns ranked LSX company rows using the recovered leaderboard ordering.</p>
        <dl class="endpoint-grid">
          <section>
            <dt>Query</dt>
            <dd><code>page</code> integer, default <code>1</code></dd>
            <dd><code>page_size</code> integer, default <code>10</code>, max <code>100</code></dd>
            <dd><code>sort</code> one of <code>market</code>, <code>company</code>, <code>ceo</code>, <code>lifespan</code></dd>
            <dd><code>username</code>, <code>gamemode</code>, <code>gamegoal</code> filters</dd>
          </section>
          <section>
            <dt>Response</dt>
            <dd><code>data[]</code> rows with <code>rank</code>, <code>company</code>, <code>ceo</code>, market value</dd>
            <dd><code>pagination</code> object with total count and page bounds</dd>
          </section>
        </dl>
      </article>

      <article class="endpoint-card">
        <header class="endpoint-head">
          <span class="endpoint-method head">HEAD</span>
          <code>/api/v1/leaderboard</code>
        </header>
        <p>Checks whether the read-only leaderboard resource is available without downloading a body.</p>
        <dl class="endpoint-grid">
          <section>
            <dt>Success</dt>
            <dd><code>200</code> leaderboard resource is present</dd>
          </section>
          <section>
            <dt>Errors</dt>
            <dd><code>404</code> not found, <code>405</code> unsupported method, <code>500</code> server error</dd>
          </section>
        </dl>
      </article>

      <article class="sample-card">
        <header class="endpoint-head">
          <span class="endpoint-method sample">JSON</span>
          <code>sample response</code>
        </header>
        <pre>{sampleResponse}</pre>
      </article>
    </section>
  </article>
</ProjectShell>
