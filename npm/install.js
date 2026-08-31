#!/usr/bin/env node
// ffwiz npm installer: downloads the platform binary from GitHub Releases.
// Zero npm dependencies; uses system `tar` (bundled on macOS, Linux, Windows 10+).
const { createReadStream, createWriteStream, existsSync, mkdirSync, chmodSync } = require("fs");
const { createHash } = require("crypto");
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

// Full version, prereleases included: goreleaser cuts a release (with
// matching checksums.txt) for every tag, and archive asset names carry no
// version, so this maps a beta package to its own release assets.
const tag = `v${version}`;
const url = `https://github.com/fsx8/ffwiz/releases/download/${tag}/${asset}`;
const checksumsUrl = `https://github.com/fsx8/ffwiz/releases/download/${tag}/checksums.txt`;
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

function downloadTo(url, filePath) {
  return get(url).then(
    (res) =>
      new Promise((resolve, reject) => {
        const ws = createWriteStream(filePath);
        res.pipe(ws);
        ws.on("finish", resolve);
        ws.on("error", reject);
      })
  );
}

function getBody(url) {
  return get(url).then(
    (res) =>
      new Promise((resolve, reject) => {
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      })
  );
}

function sha256sum(filePath) {
  return new Promise((resolve, reject) => {
    const hash = createHash("sha256");
    createReadStream(filePath)
      .on("data", (chunk) => hash.update(chunk))
      .on("end", () => resolve(hash.digest("hex")))
      .on("error", reject);
  });
}

// checksums.txt is a sha256sum-style list: "<hash>  <filename>".
function expectedChecksum(text, name) {
  for (const line of text.split(/\r?\n/)) {
    const [hash, file] = line.trim().split(/\s+/);
    if (file === name) return hash.toLowerCase();
  }
  return null;
}

(async () => {
  try {
    console.log(`ffwiz: downloading ${asset}`);
    await downloadTo(url, archive);
    console.log("ffwiz: verifying checksum");
    const expected = expectedChecksum((await getBody(checksumsUrl)).toString("utf8"), asset);
    if (!expected) {
      throw new Error(`checksums.txt from ${tag} has no entry for ${asset}`);
    }
    const actual = await sha256sum(archive);
    if (actual !== expected) {
      throw new Error(`checksum mismatch for ${asset}: expected ${expected}, got ${actual}`);
    }
    execFileSync("tar", ["-xzf", archive, "-C", binDir], { stdio: "inherit" });
    chmodSync(binPath, 0o755);
    console.log(`ffwiz: installed ${binName}`);
  } catch (err) {
    console.error(`ffwiz: install failed: ${err.message}`);
    console.error("ffwiz: you can download manually from https://github.com/fsx8/ffwiz/releases");
    process.exit(1);
  }
})();
