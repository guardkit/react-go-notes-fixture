// Generates apps/web/src/api-types.ts from contract/openapi.yaml with the
// pinned openapi-typescript. With --check it regenerates in memory and fails
// when the committed file differs (contract drift).
import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
import openapiTS, { astToString } from "openapi-typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "..", "..");
const source = path.join(root, "contract", "openapi.yaml");
const target = path.join(root, "apps", "web", "src", "api-types.ts");
const header =
  "// Generated from contract/openapi.yaml by openapi-typescript 7.13.0.\n" +
  "// Do not edit: run `npm run contract:generate` after changing the contract.\n\n";

const ast = await openapiTS(pathToFileURL(source), { alphabetize: true });
const generated = header + astToString(ast);

if (process.argv.includes("--check")) {
  let committed = "";
  try {
    committed = await readFile(target, "utf8");
  } catch {
    committed = "";
  }
  if (committed !== generated) {
    console.error(
      "contract drift: apps/web/src/api-types.ts does not match contract/openapi.yaml; " +
        "run `npm run contract:generate` and commit the result",
    );
    process.exit(1);
  }
  console.log("contract check: api-types.ts matches contract/openapi.yaml");
} else {
  await writeFile(target, generated);
  console.log(`wrote ${path.relative(root, target)}`);
}
