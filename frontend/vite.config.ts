import { defineConfig, Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { readFileSync, writeFileSync, readdirSync, statSync } from "fs";
import { join } from "path";

/**
 * Wails generates .js binding files with `// @ts-check`, which causes
 * TypeScript errors in the editor (implicit any, etc.). This plugin
 * patches them to `// @ts-nocheck` on startup and whenever Wails
 * regenerates them during `wails dev`.
 */
function patchWailsJsBindings(): Plugin {
  const patch = (file: string) => {
    const content = readFileSync(file, "utf-8");
    if (content.startsWith("// @ts-check")) {
      writeFileSync(file, content.replace("// @ts-check", "// @ts-nocheck"));
      console.log(`[wails-patch] patched ${file}`);
    }
  };

  const findJsFiles = (dir: string): string[] => {
    const results: string[] = [];
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const fullPath = join(dir, entry.name);
      if (entry.isDirectory()) {
        results.push(...findJsFiles(fullPath));
      } else if (entry.name.endsWith(".js")) {
        results.push(fullPath);
      }
    }
    return results;
  };

  return {
    name: "patch-wails-js-bindings",
    buildStart() {
      const wailsjsDir = join(process.cwd(), "wailsjs");
      findJsFiles(wailsjsDir).forEach(patch);
    },
    configureServer(server) {
      server.watcher.add(join(process.cwd(), "wailsjs"));
      server.watcher.on("change", (file) => {
        if (file.includes("wailsjs") && file.endsWith(".js")) {
          patch(file);
        }
      });
    },
  };
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [patchWailsJsBindings(), react()],
});
