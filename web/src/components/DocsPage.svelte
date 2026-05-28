<script lang="ts">
	import { onMount } from "svelte";
	import createClient from "openapi-fetch";
	import redocUrl from "redoc/bundles/redoc.standalone.js?url";
	import ProjectShell from "./ProjectShell.svelte";
	import type { paths } from "../openapi-types";
	import type { ProjectData } from "../types";

  export let data: ProjectData;

  const api = createClient<paths>({ baseUrl: "" });
  const redocOptions = {
    hideHostname: true,
    nativeScrollbars: true,
    scrollYOffset: 12,
  };

	let redocElement: HTMLElement;
	let docsError = "";
	let sampleRows: number | null = null;
	let sampleError = "";
	let redocScript: Promise<void> | null = null;

  onMount(() => {
    void mountRedoc();
    void loadSample();
  });

	async function mountRedoc() {
		try {
			await loadRedocScript();
			window.Redoc.init("/openapi.yaml", redocOptions, redocElement);
		} catch (caught) {
			docsError = caught instanceof Error ? caught.message : "unknown error";
		}
	}

	function loadRedocScript() {
		redocScript ??= new Promise((resolve, reject) => {
			const script = document.createElement("script");
			script.src = redocUrl;
			script.onload = () => resolve();
			script.onerror = () => reject(new Error("Unable to load Redoc"));
			document.head.append(script);
		});

		return redocScript;
	}

  async function loadSample() {
    const { data: leaderboard, error } = await api.GET("/api/v1/leaderboard", {
      params: { query: { page_size: 3 } },
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
        <p class="docs-copy">Live contract documentation and a typed leaderboard sample.</p>
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

    {#if docsError}
      <p class="status-message error">OpenAPI documentation could not be loaded: {docsError}</p>
    {/if}

    <section bind:this={redocElement} class="redoc-panel" aria-label="OpenAPI reference"></section>
  </article>
</ProjectShell>
