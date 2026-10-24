import { it } from "vitest";
import { execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

it("installs the packed dependency tree offline and renders both adapters with host Preact", async () => {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
  const temp = await mkdtemp(resolve(tmpdir(), "formgen-pack-test-"));
  try {
    const [pack] = JSON.parse(execFileSync("npm", ["pack", "--offline", "--json", "--pack-destination", temp], {
      cwd: resolve(root, "client"), encoding: "utf8", maxBuffer: 16 * 1024 * 1024,
    }));
    const manifest = JSON.parse(await readFile(resolve(root, "client/runtime-manifest.json"), "utf8"));
    execFileSync(process.execPath, [resolve(root, "scripts/check-package.mjs"), resolve(temp, pack.filename),
      manifest.version, manifest.fingerprint, resolve(root, "client/runtime-manifest.json")], {
      encoding: "utf8", maxBuffer: 16 * 1024 * 1024,
    });
  } finally { await rm(temp, { recursive: true, force: true }); }
}, 30000);
