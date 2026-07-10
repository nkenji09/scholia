#!/usr/bin/env node
// Thin shim: forwards argv straight through to the platform binary that
// install.js downloaded into this same directory during postinstall.
"use strict";

const path = require("path");
const { spawnSync } = require("child_process");

const binaryName = process.platform === "win32" ? "pmem.exe" : "pmem";
const binaryPath = path.join(__dirname, binaryName);

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
});

if (result.error) {
  if (result.error.code === "ENOENT") {
    console.error(
      `pmem: binary not found at ${binaryPath}. Try reinstalling: npm install product-memory`
    );
  } else {
    console.error(`pmem: failed to launch binary: ${result.error.message}`);
  }
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
