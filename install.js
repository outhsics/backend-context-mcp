const https = require("https");
const fs = require("fs");
const path = require("path");

const pkg = require("./package.json");
const VERSION = "v" + pkg.version;
const REPO = "outhsics/backend-context-mcp";

const PLATFORM_MAP = {
  "darwin-arm64": "darwin-arm64",
  "darwin-x64": "darwin-amd64",
  "linux-x64": "linux-amd64",
  "linux-arm64": "linux-arm64",
  "win32-x64": "windows-amd64.exe",
};

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (u) => {
      https
        .get(u, { headers: { "User-Agent": "node" } }, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            follow(res.headers.location);
            return;
          }
          if (res.statusCode !== 200) {
            reject(new Error("HTTP " + res.statusCode));
            return;
          }
          const file = fs.createWriteStream(dest);
          res.pipe(file);
          file.on("finish", () => {
            file.close();
            resolve();
          });
          file.on("error", reject);
        })
        .on("error", reject);
    };
    follow(url);
  });
}

async function main() {
  const key = process.platform + "-" + process.arch;
  const target = PLATFORM_MAP[key];
  if (!target) {
    console.error("Unsupported platform: " + key);
    process.exit(1);
  }

  const binDir = path.join(__dirname, "bin");
  const binFile = path.join(binDir, "byjyedu-backend-context-" + target);

  if (fs.existsSync(binFile)) {
    console.log("Binary already exists, skipping download.");
    return;
  }

  fs.mkdirSync(binDir, { recursive: true });

  const url =
    "https://github.com/" + REPO + "/releases/download/" + VERSION + "/byjyedu-backend-context-" + target;
  console.log("Downloading " + target + " ...");

  try {
    await download(url, binFile);
    fs.chmodSync(binFile, 0o755);
    console.log("Done!");
  } catch (err) {
    console.error("Download failed: " + err.message);
    process.exit(1);
  }
}

main();
