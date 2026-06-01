import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig, type Plugin } from "vite";

const configDir = dirname(fileURLToPath(import.meta.url));

const projectAssetDevPaths: Record<string, string> = {
  "flameflag_lemon.png": "static/project/flameflag_lemon.png",
  "lt2_asset_credits.avif": "static/project/lt2_asset_credits.avif",
  "lt2_asset_github.avif": "static/project/lt2_asset_github.avif",
  "lt2_green_pill.avif": "static/project/lt2_green_pill.avif",
  "lt2_icon_findings.avif": "static/project/lt2_icon_findings.avif",
  "lt2_icon_lsx.avif": "static/project/lt2_icon_lsx.avif",
  "lt2_icon_play.avif": "static/project/lt2_icon_play.avif",
  "lt2_lemon_pair.avif": "static/project/lt2_lemon_pair.avif",
  "lt2_logo_text_only.avif": "static/project/lt2_logo_text_only.avif",
  "menu_map_backdrop.avif": "static/project/menu_map_backdrop.avif",
  "pitcher.avif": "static/admin/pitcher.avif",
  "timz_lemon.png": "static/project/timz_lemon.png",
  "warning.avif": "static/admin/warning.avif"
};

function lsxProjectAssetDevServer(): Plugin {
  return {
    name: "lsx-project-asset-dev-server",
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const requestUrl = req.originalUrl ?? req.url ?? "/";
        const pathname = new URL(requestUrl, "http://127.0.0.1").pathname;
        const prefix = "/project/asset/";

        if (!pathname.startsWith(prefix)) {
          next();
          return;
        }

        const name = decodeURIComponent(pathname.slice(prefix.length));
        const staticPath = projectAssetDevPaths[name];
        if (!staticPath) {
          next();
          return;
        }

        try {
          const data = await readFile(resolve(configDir, staticPath));
          res.statusCode = 200;
          res.setHeader("Cache-Control", "no-cache");
          res.setHeader("Content-Type", assetContentType(name));
          if (req.method !== "HEAD") {
            res.end(data);
            return;
          }
          res.end();
        } catch {
          next();
        }
      });
    }
  };
}

function assetContentType(name: string) {
  if (name.endsWith(".avif")) return "image/avif";
  if (name.endsWith(".png")) return "image/png";
  return "application/octet-stream";
}

export default defineConfig({
  base: "/project/asset/svelte/",
  plugins: [lsxProjectAssetDevServer(), svelte(), tailwindcss()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    manifest: "manifest.json",
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      input: resolve(configDir, "src/main.ts"),
      output: {
        assetFileNames: "[name][extname]",
        chunkFileNames: "[name].js",
        entryFileNames: "main.js"
      }
    }
  }
});
