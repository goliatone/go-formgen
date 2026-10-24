import { expect, it } from "vitest";
import { execFileSync } from "node:child_process";
import { constants } from "node:fs";
import { cp, copyFile, mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { buildInfo, buildInputs, copies, files, manifestPath } from "../scripts/artifacts.mjs";

it("installs the packed dependency tree offline and renders both adapters with host Preact", async () => {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
  const temp = await mkdtemp(resolve(tmpdir(), "formgen-pack-test-"));
  try {
    const readOptional = (path: string) => readFile(resolve(root, path)).catch(error => {
      if (error.code === "ENOENT") return null;
      throw error;
    });
    const originalDist = await files(root, "client/dist").catch(error => {
      if (error.code === "ENOENT") return [];
      throw error;
    });
    const outputs = [...new Set([manifestPath, ...Object.values(copies).flat(),
      ...originalDist])];
    const before = await Promise.all(outputs.map(readOptional));
    const sourceIdentity = await buildInfo(root);

    // Build current sources in a private tree. A developer checkout may have no
    // dist/ or a stale release manifest after any fingerprinted source edit.
    const candidate = resolve(temp, "candidate");
    for (const path of await buildInputs(root)) {
      const target = resolve(candidate, path);
      await mkdir(dirname(target), { recursive: true });
      await copyFile(resolve(root, path), target);
    }
    for (const path of Object.values(copies).flat()) {
      await mkdir(dirname(resolve(candidate, path)), { recursive: true });
    }
    // Reuse the preflight-checked install; no network or dependency reinstall.
    await cp(resolve(root, "client/node_modules"), resolve(candidate, "client/node_modules"), {
      recursive: true, verbatimSymlinks: true, mode: constants.COPYFILE_FICLONE,
    });
    execFileSync("npm", ["run", "build", "--offline"], {
      cwd: resolve(candidate, "client"), encoding: "utf8", maxBuffer: 16 * 1024 * 1024,
    });
    const [pack] = JSON.parse(execFileSync("npm", ["pack", "--offline", "--json", "--pack-destination", temp], {
      cwd: resolve(candidate, "client"), encoding: "utf8", maxBuffer: 16 * 1024 * 1024,
    }));
    const manifest = JSON.parse(await readFile(resolve(candidate, manifestPath), "utf8"));
    expect(manifest).toMatchObject(sourceIdentity);
    execFileSync(process.execPath, [resolve(candidate, "scripts/check-package.mjs"), resolve(temp, pack.filename),
      manifest.version, manifest.fingerprint, resolve(candidate, manifestPath)], {
      encoding: "utf8", maxBuffer: 16 * 1024 * 1024,
    });
    expect(await Promise.all(outputs.map(readOptional))).toEqual(before);
  } finally { await rm(temp, { recursive: true, force: true }); }
}, 60000);
