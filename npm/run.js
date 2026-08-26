#!/usr/bin/env node
// Launches the downloaded platform binary (installed by install.js).
const { spawnSync } = require("child_process");
const { platform } = require("os");
const { join } = require("path");

const binName = platform() === "win32" ? "ffwiz.exe" : "ffwiz";
const binPath = join(__dirname, "bin", binName);

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 1);
