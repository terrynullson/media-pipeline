import { fileURLToPath } from "url";
import { dirname } from "path";
process.chdir(dirname(fileURLToPath(import.meta.url)));
await import("./node_modules/vite/bin/vite.js");
