#!/usr/bin/env node
// ffwiz npm installer: downloads the platform binary from GitHub Releases.
// Zero npm dependencies; uses system `tar` (bundled on macOS, Linux, Windows 10+).
const { createWriteStream, existsSync, mkdirSync, chmodSync } = require("fs");
const { execFileSync } = require("child_process");
const { platform, arch } = require("os");
const { join } = require("path");
const https = require("https");
const { version } = require("./package.json");

const PLATFORMS = {
  "darwin arm64": "ffwiz_darwin_arm64.tar.gz",
  "darwin x64": "ffwiz_darwin_amd64.tar.gz",
  "linux arm64": "ffwiz_linux_arm64.tar.gz",
  "linux x64": "ffwiz_linux_amd64.tar.gz",
  "win32 arm64": "ffwiz_windows_arm64.zip",
  "win32 x64": "ffwiz_windows_amd64.zip",
};

const key = `${platform()} ${arch()}`;
const asset = PLATFORMS[key];
if (!asset) {
  console.error(`ffwiz: no prebuilt binary for ${key}; install ffmpeg from source instead.`);
  process.exit(1);
}

const tag = `v${version.replace(/-.*/, "")}`;
const url = `https://github.com/fsx8/ffwiz/releases/download/${tag}/${asset}`;
const binDir = join(__dirname, "bin");
const binName = platform() === "win32" ? "ffwiz.exe" : "ffwiz";
const binPath = join(binDir, binName);

if (existsSync(binPath)) process.exit(0);

mkdirSync(binDir, { recursive: true });
const archive = join(binDir, asset);

function get(url, redirects = 5) {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { "user-agent": "ffwiz-npm-installer" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location && redirects > 0) {
          res.resume();
          return get(res.headers.location, redirects - 1).then(resolve, reject);
        }
        if (res.statusCode !== 200) {
          res.resume();
          return reject(new Error(`download failed: HTTP ${res.statusCode} for ${url}`));
        }
        resolve(res);
      })
      .on("error", reject);
  });
}

(async () => {
  try {
    console.log(`ffwiz: downloading ${asset}`);
    const res = await get(url);
    await new Promise((resolve, reject) => {
      const ws = createWriteStream(archive);
      res.pipe(ws);
      ws.on("finish", resolve);
      ws.on("error", reject);
    });
    execFileSync("tar", ["-xzf", archive, "-C", binDir], { stdio: "inherit" });
    chmodSync(binPath, 0o755);
    console.log(`ffwiz: installed ${binName}`);
  } catch (err) {
    console.error(`ffwiz: install failed: ${err.message}`);
    console.error("ffwiz: you can download manually from https://github.com/fsx8/ffwiz/releases");
    process.exit(1);
  }
})();
