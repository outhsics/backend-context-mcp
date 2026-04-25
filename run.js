#!/usr/bin/env node
const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const binDir = path.join(__dirname, "bin");

if (!fs.existsSync(binDir)) {
  console.error("Binary not found. Run: node install.js");
  process.exit(1);
}

const files = fs.readdirSync(binDir);
const binary = files.find((f) => f.startsWith("byjyedu-backend-context"));
if (!binary) {
  console.error("Binary not found. Run: node install.js");
  process.exit(1);
}

try {
  execFileSync(path.join(binDir, binary), process.argv.slice(2), {
    stdio: "inherit",
  });
} catch (e) {
  process.exit(e.status || 1);
}
