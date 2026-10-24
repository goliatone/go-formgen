// @vitest-environment node
import { afterEach, expect, it } from "vitest";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, dirname } from "node:path";
import { assertInstalledDependencies } from "../scripts/check-dependencies.mjs";

const temporary: string[] = [];
afterEach(async () => { await Promise.all(temporary.splice(0).map(path => rm(path, { recursive: true, force: true }))); });

async function fixture() {
  const client = await mkdtemp(resolve(tmpdir(), "formgen-dependencies-"));
  temporary.push(client);
  const pkg = { dependencies: { example: "1.0.0" } };
  await writeFile(resolve(client, "package.json"), JSON.stringify(pkg));
  await writeFile(resolve(client, "package-lock.json"), JSON.stringify({ packages: {
    "": pkg, "node_modules/example": { version: "1.0.0" },
    "node_modules/optional-platform": { version: "2.0.0", optional: true },
    "node_modules/example/node_modules/transitive": { version: "3.0.0" },
  } }));
  for (const [path, version] of [["example", "1.0.0"], ["example/node_modules/transitive", "3.0.0"]]) {
    const file = resolve(client, "node_modules", path, "package.json");
    await mkdir(dirname(file), { recursive: true });
    await writeFile(file, JSON.stringify({ version }));
  }
  return client;
}

it("accepts the locked tree with absent optional platform packages", async () => {
  await assertInstalledDependencies(await fixture());
});
it("reports stale direct and transitive packages with a reinstall instruction", async () => {
  const client = await fixture();
  await writeFile(resolve(client, "node_modules/example/package.json"), '{"version":"0.9.0"}');
  await writeFile(resolve(client, "node_modules/example/node_modules/transitive/package.json"), '{"version":"2.0.0"}');
  await expect(assertInstalledDependencies(client)).rejects.toThrow(/installed 0.9.0, locked 1.0.0[\s\S]*installed 2.0.0, locked 3.0.0[\s\S]*npm ci --ignore-scripts/);
});
it("rejects missing required packages and drifted root requirements", async () => {
  const client = await fixture();
  await rm(resolve(client, "node_modules/example/node_modules/transitive/package.json"));
  await expect(assertInstalledDependencies(client)).rejects.toThrow(/transitive: missing/);
  await writeFile(resolve(client, "package.json"), '{"dependencies":{"example":"2.0.0"}}');
  await expect(assertInstalledDependencies(client)).rejects.toThrow(/package.json dependencies differs/);
});
