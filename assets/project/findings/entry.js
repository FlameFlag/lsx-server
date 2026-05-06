import { highlightCodeBlocks } from "./code.js";

const ROOT_ID = "findings-blog-root";

const render = () => {
  const root = document.getElementById(ROOT_ID);
  if (!root) {
    return;
  }

  highlightCodeBlocks(root);
};

const onReady = (callback) => {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", callback, { once: true });
    return;
  }

  callback();
};

onReady(render);
