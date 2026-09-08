import fs from "node:fs";
import path from "node:path";
const files = [];
function walk(dir) {
  for (const item of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, item.name);
    if (item.isDirectory()) walk(p);
    else files.push(p);
  }
}
walk("src");
const errors = [];
for (const file of files) {
  if (/\.(woff2?|ttf|otf|ttc)$/.test(file))
    errors.push(`${file}: do not redistribute system font files`);
  if (!/\.(vue|ts)$/.test(file)) continue;
  const text = fs.readFileSync(file, "utf8");
  if (file.endsWith(".vue") && !text.includes('<script setup lang="ts">'))
    errors.push(`${file}: Vue SFC must use typed script setup`);
  if (text.includes("v-html=")) errors.push(`${file}: raw HTML rendering is forbidden`);
  const imports = [...text.matchAll(/from\s+["']([^"']+)["']/g)].map((m) => m[1]);
  if (
    file.startsWith("src/shared/") &&
    imports.some(
      (i) =>
        i.startsWith("@/features/") ||
        i.startsWith("@/app/") ||
        i.startsWith("@/services/"),
    )
  )
    errors.push(`${file}: shared layer may not depend on business or app layers`);
  if (file.startsWith("src/features/") && imports.some((i) => i.startsWith("@/app/")))
    errors.push(`${file}: features may not depend on app shell`);
  if (file.includes("/pages/") && imports.some((i) => i.includes("/pages/")))
    errors.push(`${file}: pages may not import other pages`);
  if (file.endsWith(".vue") && text.split("\n").length > 420)
    errors.push(`${file}: split this SFC; limit is 420 formatted lines`);
}
if (fs.readFileSync("src/App.vue", "utf8").split("\n").length > 40)
  errors.push("App.vue must remain composition-only");
if (errors.length) {
  console.error(errors.join("\n"));
  process.exit(1);
}
console.log(
  `Architecture check passed: ${files.length} source files; typed SFCs; explicit layer boundaries; no oversized pages or bundled fonts.`,
);
