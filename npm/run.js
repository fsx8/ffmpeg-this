#!/usr/bin/env node
// Launches the downloaded platform binary (installed by install.js).
const { spawnSync } = require("child_process");
const { platform } = require("os");
const { join } = require("path");

const binName = platform() === "win32" ? "ffwiz.exe" : "ffwiz";
const binPath = join(__dirname, "bin", binName);

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
if (result.error || result.status === null) {
  console.error(`ffwiz: failed to launch ${binPath}${result.error ? `: ${result.error.message}` : ""}`);
  console.error("ffwiz: the binary download may have failed during install — run 'npm rebuild ffwiz', or reinstall the package.");
  process.exit(1);
}
process.exit(result.status);
