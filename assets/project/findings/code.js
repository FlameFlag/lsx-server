const SHIKI_VENDOR_URL = "/project/asset/findings/vendor/shiki/";
const SHIKI_UNPKG_URL = `${SHIKI_VENDOR_URL}unpkg/`;
const SHIKI_CORE_URL = `${SHIKI_UNPKG_URL}shiki@4.0.2/dist/core.mjs`;
const SHIKI_ENGINE_URL =
  `${SHIKI_UNPKG_URL}shiki@4.0.2/dist/engine-oniguruma.mjs`;
const SHIKI_WASM_URL = `${SHIKI_UNPKG_URL}shiki@4.0.2/dist/wasm.mjs`;
const SHIKI_THEME_URL =
  `${SHIKI_UNPKG_URL}@shikijs/themes@4.0.2/dist/github-light.mjs`;
const SHIKI_THEME = "github-light";

const SUPPORTED_LANGUAGES = ["bash", "c", "http", "python"];
const LANGUAGE_URLS = {
  bash: `${SHIKI_UNPKG_URL}@shikijs/langs@4.0.2/dist/shellscript.mjs`,
  c: `${SHIKI_UNPKG_URL}@shikijs/langs@4.0.2/dist/c.mjs`,
  http: `${SHIKI_UNPKG_URL}@shikijs/langs@4.0.2/dist/http.mjs`,
  python: `${SHIKI_UNPKG_URL}@shikijs/langs@4.0.2/dist/python.mjs`,
};

const LANGUAGE_SET = new Set(SUPPORTED_LANGUAGES);

const LANGUAGE_RULES = [
  {
    language: "bash",
    pattern:
      /^\s*(INPUT=|OUT=|mkdir\s+|sha256sum\s+|file\s+|binwalk\s+|strings\s+|dd\s+|bzip2\s+|python3\s+|go\s+|curl\s+)/m,
  },
  {
    language: "python",
    pattern:
      /^\s*(import\s+struct|from\s+\S+\s+import|def\s+|elif\s+|raise\s+ValueError)/m,
  },
  {
    language: "c",
    pattern:
      /^\s*(struct\s+\w+|int\s+\w+\s*\(|account_ok\s*=|upload_ok\s*=|checksum\s*=|gamestartingdate\s*=)/m,
  },
  {
    language: "http",
    pattern: /^\s*GET\s+\//,
  },
];

let shikiHighlighterPromise = null;

const scheduleIdleWork = (callback) => {
  if ("requestIdleCallback" in window) {
    window.requestIdleCallback(callback, { timeout: 500 });
    return;
  }

  window.setTimeout(() => callback({ timeRemaining: () => 8 }), 0);
};

const detectLanguage = (pre) =>
  pre.getAttribute("data-code-language") ||
  LANGUAGE_RULES.find(({ pattern }) => pattern.test(pre.textContent))?.language ||
  "";

const loadShikiHighlighter = () => {
  shikiHighlighterPromise ||= Promise.all([
    import(SHIKI_CORE_URL),
    import(SHIKI_ENGINE_URL),
    import(SHIKI_THEME_URL),
    ...SUPPORTED_LANGUAGES.map((language) => import(LANGUAGE_URLS[language])),
  ]).then(([core, engine, theme, ...languages]) =>
    core.createHighlighterCore({
      themes: [theme.default],
      langs: languages.map((language) => language.default),
      engine: engine.createOnigurumaEngine(import(SHIKI_WASM_URL)),
    }),
  );

  return shikiHighlighterPromise;
};

const highlightedNode = (html, language) => {
  const template = document.createElement("template");
  template.innerHTML = html.trim();

  const pre = template.content.firstElementChild;
  if (!pre || pre.tagName !== "PRE") {
    return null;
  }

  pre.classList.add("findings-code", "highlighted");
  pre.setAttribute("data-code-language", language);
  return pre;
};

const highlightCodeBlock = (highlighter, pre) => {
  const language = detectLanguage(pre);
  if (!LANGUAGE_SET.has(language)) {
    return;
  }

  try {
    const html = highlighter.codeToHtml(pre.textContent, {
      lang: language,
      theme: SHIKI_THEME,
    });
    const next = highlightedNode(html, language);
    if (next) {
      pre.replaceWith(next);
    }
  } catch (_) {
    // Keep the readable plain block if the highlighter or grammar fails.
  }
};

export const highlightCodeBlocks = async (root) => {
  const blocks = [...root.querySelectorAll(".findings-code")].filter((pre) =>
    LANGUAGE_SET.has(detectLanguage(pre)),
  );
  if (blocks.length === 0) {
    return;
  }

  try {
    const highlighter = await loadShikiHighlighter();
    let index = 0;
    const highlightNext = (deadline) => {
      while (
        index < blocks.length &&
        (deadline.timeRemaining() > 4 || index === 0)
      ) {
        if (blocks[index].isConnected) {
          highlightCodeBlock(highlighter, blocks[index]);
        }
        index += 1;
      }

      if (index < blocks.length) {
        scheduleIdleWork(highlightNext);
      }
    };

    scheduleIdleWork(highlightNext);
  } catch (_) {
    // Plain code blocks are still usable without the optional highlighter.
  }
};
