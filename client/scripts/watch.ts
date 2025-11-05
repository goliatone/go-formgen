import { spawn } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const binExtension = process.platform === "win32" ? ".cmd" : "";
const tsxBin = resolve(projectRoot, "node_modules", ".bin", `tsx${binExtension}`);

const processes = [
  spawn(tsxBin, ["scripts/build-theme.ts", "--watch"], {
    cwd: projectRoot,
    stdio: "inherit",
  }),
  spawn(tsxBin, ["scripts/build.ts", "--watch"], {
    cwd: projectRoot,
    stdio: "inherit",
  }),
];

function terminate(signal: NodeJS.Signals = "SIGTERM") {
  for (const child of processes) {
    if (!child.killed) {
      child.kill(signal);
    }
  }
}

process.on("SIGINT", () => {
  terminate("SIGINT");
  process.exit(0);
});

process.on("SIGTERM", () => {
  terminate("SIGTERM");
  process.exit(0);
});

let exiting = false;
for (const child of processes) {
  child.on("exit", (code) => {
    if (exiting) {
      return;
    }
    if (code !== 0) {
      exiting = true;
      terminate("SIGTERM");
      process.exit(code ?? 1);
    }
  });
}
