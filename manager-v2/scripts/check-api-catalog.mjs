import { readFileSync } from "node:fs";

const routesSource = readFileSync(new URL("../../pkg/routes/routes.go", import.meta.url), "utf8");
const catalogSource = readFileSync(new URL("../src/api-catalog.ts", import.meta.url), "utf8");

const ignored = new Set([
  "/favicon.ico",
  "/manager",
  "/manager/*any",
]);

const routePaths = new Set();
let currentGroup = "";
for (const line of routesSource.split(/\r?\n/)) {
  const groupMatch = line.match(/routes\s*(?::=|=)\s*eng\.Group\("([^"]+)"\)/);
  if (groupMatch) {
    currentGroup = groupMatch[1];
    continue;
  }

  const groupedRoute = line.match(/routes\.(?:GET|POST|PUT|PATCH|DELETE)\("([^"]+)"/);
  if (groupedRoute) {
    routePaths.add(`${currentGroup}${groupedRoute[1]}`);
    continue;
  }

  const directRoute = line.match(/eng\.(?:GET|POST|PUT|PATCH|DELETE)\("([^"]+)"/);
  if (directRoute && !directRoute[1].startsWith("/swagger") && !ignored.has(directRoute[1])) {
    routePaths.add(directRoute[1]);
  }
}

const catalogPaths = new Set(
  Array.from(catalogSource.matchAll(/"(\/[^"\s]+)"/g), (match) => match[1]),
);

const missing = Array.from(routePaths).filter((path) => !catalogPaths.has(path)).sort();
if (missing.length > 0) {
  console.error("API Lab is missing registered routes:");
  missing.forEach((path) => console.error(`- ${path}`));
  process.exit(1);
}

console.log(`API catalog covers ${routePaths.size} registered routes.`);
