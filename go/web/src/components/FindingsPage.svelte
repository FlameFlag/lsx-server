<script lang="ts">
import SvelteMarkdown from "@humanspeak/svelte-markdown";
import { Buffer } from "buffer";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import c from "highlight.js/lib/languages/c";
import go from "highlight.js/lib/languages/go";
import http from "highlight.js/lib/languages/http";
import javascript from "highlight.js/lib/languages/javascript";
import python from "highlight.js/lib/languages/python";
import shell from "highlight.js/lib/languages/shell";
import typescript from "highlight.js/lib/languages/typescript";
import { writable } from "svelte/store";
import slugify from "slugify";
import type { ProjectData } from "../types";
import ProjectShell from "./ProjectShell.svelte";

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
  bodyBlocks: FindingBodyBlock[];
  headings: FindingHeading[];
};

type ImplementationLanguage = "go" | "c" | "python" | "ts";
type MarkdownBodyBlock = {
  type: "markdown";
  id: string;
  source: string;
};
type CodeBodyBlock = {
  type: "code";
  id: string;
  lang: string;
  text: string;
};
type CodeGroupBodyBlock = {
  type: "codeGroup";
  id: string;
  blocks: CodeBodyBlock[];
};
type FindingBodyBlock = MarkdownBodyBlock | CodeBodyBlock | CodeGroupBodyBlock;

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("c", c);
hljs.registerLanguage("go", go);
hljs.registerLanguage("http", http);
hljs.registerLanguage("javascript", javascript);
hljs.registerLanguage("js", javascript);
hljs.registerLanguage("python", python);
hljs.registerLanguage("shell", shell);
hljs.registerLanguage("sh", shell);
hljs.registerLanguage("typescript", typescript);
hljs.registerLanguage("ts", typescript);

const markdownOptions = { headerIds: false };
const implementationLanguages: { id: ImplementationLanguage; label: string }[] = [
  { id: "go", label: "Go" },
  { id: "c", label: "C" },
  { id: "python", label: "Python" },
  { id: "ts", label: "TS" }
];
const globals = globalThis as typeof globalThis & { Buffer?: typeof Buffer };

globals.Buffer ??= Buffer;

const selectedCodeLanguages = writable<Record<string, ImplementationLanguage>>({});

$: article = parseFindings(data.markdown || "");

function parseFindings(source: string): FindingArticle | null {
  const parsed = parseFrontmatter(source);
  if (!parsed.metadata.title) return null;
  return { ...parsed.metadata, ...contentSections(parsed.content) };
}

function parseFrontmatter(source: string) {
  if (!source.startsWith("---\n")) return { metadata: emptyMetadata(), content: source };

  const end = source.indexOf("\n---", 4);
  if (end < 0) return { metadata: emptyMetadata(), content: source };

  return {
    metadata: metadataFromYAML(source.slice(4, end)),
    content: source.slice(end + 4).replace(/^\r?\n/, "")
  };
}

function emptyMetadata(): Required<FindingsMatter> {
  return { kicker: "", title: "", updated: "" };
}

function metadataFromYAML(frontmatter: string): Required<FindingsMatter> {
  const metadata = emptyMetadata();
  for (const line of frontmatter.split(/\r?\n/)) {
    const match = /^(kicker|title|updated):\s*(.*)$/.exec(line);
    if (!match) continue;
    const [, key, value] = match;
    metadata[key as keyof Required<FindingsMatter>] = value.replace(/^['"]|['"]$/g, "").trim();
  }
  return metadata;
}

function contentSections(content: string) {
  const bodyStart = content.search(/^##\s+/m);
  const body = stripHeadingIds(sliceMarkdown(content, bodyStart));
  return {
    intro: sliceMarkdown(content, 0, bodyStart),
    body,
    bodyBlocks: splitMarkdownCodeBlocks(body),
    headings: collectHeadings(content)
  };
}

function sliceMarkdown(source: string, start: number, end?: number) {
  if (start < 0) return "";
  return source.slice(start, end).trim();
}

function collectHeadings(source: string): FindingHeading[] {
  return Array.from(source.matchAll(/^##\s+(.+?)(?:\s+\{#([A-Za-z0-9_-]+)\})?\s*$/gm), ([, title, id]) => ({
    id: id || slugFor(title),
    title: title.trim()
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

function splitMarkdownCodeBlocks(source: string): FindingBodyBlock[] {
  const fence = /^```([^\r\n`]*)\r?\n([\s\S]*?)^```[ \t]*$/gm;
  const rawBlocks: FindingBodyBlock[] = [];
  let cursor = 0;
  let codeIndex = 0;

  for (const match of source.matchAll(fence)) {
    const start = match.index || 0;
    if (start > cursor) {
      rawBlocks.push({ type: "markdown", id: `markdown-${rawBlocks.length}`, source: source.slice(cursor, start) });
    }
    rawBlocks.push({
      type: "code",
      id: `code-${codeIndex}`,
      lang: fenceLanguage(match[1]),
      text: match[2].replace(/\r?\n$/, "")
    });
    codeIndex += 1;
    cursor = start + match[0].length;
  }

  if (cursor < source.length) {
    rawBlocks.push({ type: "markdown", id: `markdown-${rawBlocks.length}`, source: source.slice(cursor) });
  }

  return groupImplementationSamples(rawBlocks);
}

function fenceLanguage(info: string) {
  return info.trim().split(/\s+/, 1)[0] || "";
}

function groupImplementationSamples(blocks: FindingBodyBlock[]): FindingBodyBlock[] {
  const grouped: FindingBodyBlock[] = [];
  let groupIndex = 0;

  for (let i = 0; i < blocks.length; ) {
    const block = blocks[i];
    if (block.type !== "code" || !isImplementationLanguage(block.lang)) {
      grouped.push(block);
      i += 1;
      continue;
    }

    const samples = [block];
    let next = i + 1;
    while (next + 1 < blocks.length) {
      const spacer = blocks[next];
      const candidate = blocks[next + 1];
      if (
        spacer.type !== "markdown" ||
        spacer.source.trim() !== "" ||
        candidate.type !== "code" ||
        !isImplementationLanguage(candidate.lang)
      ) {
        break;
      }
      samples.push(candidate);
      next += 2;
    }

    if (samples.length > 1) {
      grouped.push({ type: "codeGroup", id: `code-group-${groupIndex}`, blocks: samples });
      groupIndex += 1;
    } else {
      grouped.push(block);
    }
    i = next;
  }

  return grouped.filter((block) => block.type !== "markdown" || block.source.trim() !== "");
}

function _highlightedCode(text: string, lang: string) {
  const language = highlightLanguage(lang);
  return language ? hljs.highlight(text, { language }).value : hljs.highlightAuto(text).value;
}

function highlightLanguage(lang: string) {
  const language = normalizeLanguage(lang);
  return language && hljs.getLanguage(language) ? language : "";
}

function normalizeLanguage(lang: string) {
  if (lang === "ts" || lang === "typescript") return "typescript";
  if (lang === "c-source") return "c";
  if (lang === "shell") return "sh";
  return lang || "";
}

function displayLanguage(lang: string) {
  if (lang === "ts" || lang === "typescript") return "TypeScript";
  if (lang === "go") return "Go";
  if (lang === "c") return "C";
  if (lang === "python") return "Python";
  if (lang === "c-source") return "Recovered C";
  if (lang === "http") return "HTTP";
  if (lang === "bash" || lang === "sh" || lang === "shell") return "Shell";
  return lang || "Text";
}

function isImplementationLanguage(lang: string): lang is ImplementationLanguage {
  return lang === "go" || lang === "c" || lang === "python" || lang === "ts";
}

function selectedGroupLanguage(group: CodeGroupBodyBlock, selections: Record<string, ImplementationLanguage>) {
  return selections[group.id] || group.blocks[0]?.lang || "go";
}

function selectGroupLanguage(group: CodeGroupBodyBlock, lang: ImplementationLanguage) {
  selectedCodeLanguages.update((selections) => ({ ...selections, [group.id]: lang }));
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
        <div class="findings-controls">
          <nav class="findings-nav" aria-label="Findings sections">
            {#each article.headings as heading}
              <a href={`#${heading.id}`}>{heading.title}</a>
            {/each}
          </nav>
        </div>
      {/if}

      <section class="findings-markdown">
        {#each article.bodyBlocks as block (block.id)}
          {#if block.type === "markdown"}
            <SvelteMarkdown source={block.source} options={markdownOptions}>
              {#snippet heading({ depth: _depth, text: _text, children: _children })}
                <svelte:element this={headingTag(_depth)} id={_depth === 2 ? sectionId(_text) : undefined}>
                  {@render _children?.()}
                </svelte:element>
              {/snippet}

              {#snippet code({ lang: _lang, text: _text })}
                <figure class="findings-code-frame">
                  <figcaption>{displayLanguage(_lang)}</figcaption>
                  <pre
                    class="findings-code"
                  ><code class={_lang ? `hljs language-${normalizeLanguage(_lang)}` : "hljs"}>{@html _highlightedCode(_text, _lang)}</code></pre>
                </figure>
              {/snippet}
            </SvelteMarkdown>
          {:else if block.type === "code"}
            <figure class="findings-code-frame" class:implementation-sample={isImplementationLanguage(block.lang)}>
              <figcaption>{displayLanguage(block.lang)}</figcaption>
              <pre
                class="findings-code"
              ><code class={block.lang ? `hljs language-${normalizeLanguage(block.lang)}` : "hljs"}>{@html _highlightedCode(block.text, block.lang)}</code></pre>
            </figure>
          {:else}
            <figure class="findings-code-frame implementation-sample">
              <figcaption class="findings-code-toolbar">
                <span>{displayLanguage(selectedGroupLanguage(block, $selectedCodeLanguages))}</span>
                <div role="group" aria-label="Choose sample language">
                  {#each implementationLanguages as language}
                    {#if block.blocks.some((sample) => sample.lang === language.id)}
                      <button
                        type="button"
                        class:active={selectedGroupLanguage(block, $selectedCodeLanguages) === language.id}
                        aria-pressed={selectedGroupLanguage(block, $selectedCodeLanguages) === language.id}
                        onclick={() => selectGroupLanguage(block, language.id)}
                      >
                        {language.label}
                      </button>
                    {/if}
                  {/each}
                </div>
              </figcaption>
              {#each block.blocks as sample (sample.id)}
                {#if selectedGroupLanguage(block, $selectedCodeLanguages) === sample.lang}
                  <pre
                    class="findings-code"
                  ><code class={`hljs language-${normalizeLanguage(sample.lang)}`}>{@html _highlightedCode(sample.text, sample.lang)}</code></pre>
                {/if}
              {/each}
            </figure>
          {/if}
        {/each}
      </section>
    {:else}
      <p class="status-message error">Findings content could not be loaded.</p>
    {/if}
  </article>
</ProjectShell>
