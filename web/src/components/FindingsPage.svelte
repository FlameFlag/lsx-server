<script lang="ts">
  import SvelteMarkdown from "@humanspeak/svelte-markdown";
  import matter from "gray-matter";
  import { Buffer } from "buffer";
  import hljs from "highlight.js/lib/core";
  import bash from "highlight.js/lib/languages/bash";
  import c from "highlight.js/lib/languages/c";
  import go from "highlight.js/lib/languages/go";
  import http from "highlight.js/lib/languages/http";
  import javascript from "highlight.js/lib/languages/javascript";
  import python from "highlight.js/lib/languages/python";
  import shell from "highlight.js/lib/languages/shell";
  import slugify from "slugify";
  import ProjectShell from "./ProjectShell.svelte";
  import type { ProjectData } from "../types";

  export let data: ProjectData;

  type FindingsMatter = {
    kicker?: string;
    title?: string;
    updated?: string;
  };

  type FindingHeading = {
    id: string;
    title: string;
  };

  type FindingArticle = FindingsMatter & {
    intro: string;
    body: string;
    headings: FindingHeading[];
  };

  hljs.registerLanguage("bash", bash);
  hljs.registerLanguage("c", c);
  hljs.registerLanguage("go", go);
  hljs.registerLanguage("http", http);
  hljs.registerLanguage("javascript", javascript);
  hljs.registerLanguage("js", javascript);
  hljs.registerLanguage("python", python);
  hljs.registerLanguage("shell", shell);
  hljs.registerLanguage("sh", shell);

  const markdownOptions = { headerIds: false };
  const globals = globalThis as typeof globalThis & { Buffer?: typeof Buffer };

  globals.Buffer ??= Buffer;

  $: article = parseFindings(data.markdown || "");

  function parseFindings(source: string): FindingArticle | null {
    const parsed = matter(source);
    const metadata = metadataFrom(parsed.data);
    if (!metadata.title) return null;
    return { ...metadata, ...contentSections(parsed.content) };
  }

  function metadataFrom(value: Record<string, unknown>): Required<FindingsMatter> {
    return {
      kicker: stringValue(value.kicker),
      title: stringValue(value.title),
      updated: stringValue(value.updated),
    };
  }

  function stringValue(value: unknown) {
    return typeof value === "string" ? value : "";
  }

  function contentSections(content: string) {
    const bodyStart = content.search(/^##\s+/m);
    return {
      intro: sliceMarkdown(content, 0, bodyStart),
      body: stripHeadingIds(sliceMarkdown(content, bodyStart)),
      headings: collectHeadings(content),
    };
  }

  function sliceMarkdown(source: string, start: number, end?: number) {
    if (start < 0) return "";
    return source.slice(start, end).trim();
  }

  function collectHeadings(source: string): FindingHeading[] {
    return Array.from(source.matchAll(/^##\s+(.+?)(?:\s+\{#([A-Za-z0-9_-]+)\})?\s*$/gm), ([, title, id]) => ({
      id: id || slugFor(title),
      title: title.trim(),
    }));
  }

  function stripHeadingIds(source: string) {
    return source.replace(/^(##\s+.+?)\s+\{#[A-Za-z0-9_-]+\}\s*$/gm, "$1");
  }

  function slugFor(value: string) {
    return slugify(value, { lower: true, strict: true });
  }

  function sectionId(title: string) {
    return article?.headings.find((heading) => heading.title === title)?.id || slugFor(title);
  }

  function headingTag(depth: number) {
    return `h${Math.min(Math.max(depth, 2), 4)}`;
  }

  function highlightedCode(text: string, lang: string) {
    const language = lang && hljs.getLanguage(lang) ? lang : "";
    return language ? hljs.highlight(text, { language }).value : hljs.highlightAuto(text).value;
  }
</script>

<ProjectShell active="findings" heading={data.heading || "Findings"}>
  <article id="findings-blog-root" class="content-card findings-root">
    {#if article}
      <header class="findings-hero">
        <p class="findings-kicker">{article.kicker}</p>
        <h2>{article.title}</h2>
        <p class="findings-updated">Updated {article.updated}</p>
        <SvelteMarkdown source={article.intro} options={markdownOptions} />
      </header>

      {#if article.headings.length}
        <nav class="findings-nav" aria-label="Findings sections">
          {#each article.headings as heading}
            <a href={`#${heading.id}`}>{heading.title}</a>
          {/each}
        </nav>
      {/if}

      <section class="findings-markdown">
        <SvelteMarkdown source={article.body} options={markdownOptions}>
          {#snippet heading({ depth, text, children })}
            <svelte:element this={headingTag(depth)} id={depth === 2 ? sectionId(text) : undefined}>
              {@render children?.()}
            </svelte:element>
          {/snippet}

          {#snippet code({ lang, text })}
            <pre class="findings-code"><code class={lang ? `language-${lang}` : ""}>{@html highlightedCode(text, lang)}</code></pre>
          {/snippet}
        </SvelteMarkdown>
      </section>
    {:else}
      <p class="status-message error">Findings content could not be loaded.</p>
    {/if}
  </article>
</ProjectShell>
