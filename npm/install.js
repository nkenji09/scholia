#!/usr/bin/env node
// postinstall: download the scholia binary for this platform/arch from GitHub
// Releases and drop it next to bin/scholia.js. No dependencies — Node stdlib only.
//
// NOTE (Phase 4 draft): this file is written as a complete implementation
// per DESIGN §10-4, but is not executed or network-tested by this task —
// there is no GitHub Release to fetch from yet.
"use strict";

const fs = require("fs");
const path = require("path");
const zlib = require("zlib");
const http = require("http");
const https = require("https");
const { URL } = require("url");

const REPO = "nkenji09/scholia";
const BINARY_NAME = "scholia";
const PKG_VERSION = require("./package.json").version;

const PLATFORM_MAP = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

function fail(message) {
  console.error(`scholia install: ${message}`);
  process.exit(1);
}

function resolveTarget() {
  const os = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];
  if (!os) fail(`unsupported platform: ${process.platform}`);
  if (!arch) fail(`unsupported architecture: ${process.arch}`);
  return { os, arch, ext: os === "windows" ? "zip" : "tar.gz" };
}

function proxyForUrl(targetUrl) {
  const isHttps = targetUrl.protocol === "https:";
  const proxy =
    (isHttps && (process.env.HTTPS_PROXY || process.env.https_proxy)) ||
    (!isHttps && (process.env.HTTP_PROXY || process.env.http_proxy));
  if (!proxy) return null;

  const noProxy = process.env.NO_PROXY || process.env.no_proxy || "";
  const bypassed = noProxy
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .some((suffix) => targetUrl.hostname.endsWith(suffix));
  return bypassed ? null : new URL(proxy);
}

// Follows redirects and (optionally) tunnels through an HTTP(S) proxy via CONNECT.
function get(targetUrl, redirectsLeft = 5) {
  return new Promise((resolve, reject) => {
    const proxy = proxyForUrl(targetUrl);

    const onResponse = (res) => {
      if (
        [301, 302, 303, 307, 308].includes(res.statusCode) &&
        res.headers.location
      ) {
        res.resume();
        if (redirectsLeft <= 0) return reject(new Error("too many redirects"));
        return resolve(
          get(new URL(res.headers.location, targetUrl), redirectsLeft - 1)
        );
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(
          new Error(`GET ${targetUrl} failed: HTTP ${res.statusCode}`)
        );
      }
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => resolve(Buffer.concat(chunks)));
      res.on("error", reject);
    };

    if (!proxy) {
      const client = targetUrl.protocol === "https:" ? https : http;
      client
        .get(targetUrl, { headers: { "user-agent": "scholia-npm-installer" } }, onResponse)
        .on("error", reject);
      return;
    }

    // Tunnel through the proxy with CONNECT, then speak TLS/HTTP over the raw socket.
    const connectReq = http.request({
      host: proxy.hostname,
      port: proxy.port || 80,
      method: "CONNECT",
      path: `${targetUrl.hostname}:${targetUrl.port || 443}`,
    });
    connectReq.on("error", reject);
    connectReq.on("connect", (res, socket) => {
      if (res.statusCode !== 200) {
        return reject(new Error(`proxy CONNECT failed: HTTP ${res.statusCode}`));
      }
      const tls = require("tls");
      const tlsSocket = tls.connect({ socket, servername: targetUrl.hostname });
      https
        .get(
          targetUrl,
          { socket: tlsSocket, agent: false, headers: { "user-agent": "scholia-npm-installer" } },
          onResponse
        )
        .on("error", reject);
    });
    connectReq.end();
  });
}

// Minimal ustar-format tar reader — enough to pull one named file out of a
// GoReleaser archive (no long-name/pax extensions expected for a single binary).
function extractFromTar(buffer, entryName) {
  let offset = 0;
  while (offset + 512 <= buffer.length) {
    const header = buffer.subarray(offset, offset + 512);
    if (header.every((b) => b === 0)) break;

    const name = header.toString("utf8", 0, 100).replace(/\0.*$/, "");
    const sizeOctal = header.toString("utf8", 124, 136).replace(/\0.*$/, "").trim();
    const size = parseInt(sizeOctal || "0", 8);
    const dataStart = offset + 512;

    if (name === entryName || name === `./${entryName}`) {
      return buffer.subarray(dataStart, dataStart + size);
    }

    offset = dataStart + Math.ceil(size / 512) * 512;
  }
  return null;
}

// Minimal ZIP reader (store + deflate methods only) — enough for GoReleaser's
// Windows archives, which contain a single small binary.
function extractFromZip(buffer, entryName) {
  const EOCD_SIG = 0x06054b50;
  let eocdOffset = -1;
  for (let i = buffer.length - 22; i >= 0; i--) {
    if (buffer.readUInt32LE(i) === EOCD_SIG) {
      eocdOffset = i;
      break;
    }
  }
  if (eocdOffset === -1) throw new Error("not a valid zip file (no EOCD record)");

  const entryCount = buffer.readUInt16LE(eocdOffset + 10);
  let cdOffset = buffer.readUInt32LE(eocdOffset + 16);

  for (let i = 0; i < entryCount; i++) {
    const sig = buffer.readUInt32LE(cdOffset);
    if (sig !== 0x02014b50) throw new Error("malformed zip central directory");
    const method = buffer.readUInt16LE(cdOffset + 10);
    const compSize = buffer.readUInt32LE(cdOffset + 20);
    const nameLen = buffer.readUInt16LE(cdOffset + 28);
    const extraLen = buffer.readUInt16LE(cdOffset + 30);
    const commentLen = buffer.readUInt16LE(cdOffset + 32);
    const localHeaderOffset = buffer.readUInt32LE(cdOffset + 42);
    const name = buffer.toString("utf8", cdOffset + 46, cdOffset + 46 + nameLen);

    if (name === entryName) {
      const lfNameLen = buffer.readUInt16LE(localHeaderOffset + 26);
      const lfExtraLen = buffer.readUInt16LE(localHeaderOffset + 28);
      const dataStart = localHeaderOffset + 30 + lfNameLen + lfExtraLen;
      const compressed = buffer.subarray(dataStart, dataStart + compSize);
      if (method === 0) return compressed;
      if (method === 8) return zlib.inflateRawSync(compressed);
      throw new Error(`unsupported zip compression method: ${method}`);
    }

    cdOffset += 46 + nameLen + extraLen + commentLen;
  }
  return null;
}

async function main() {
  const { os, arch, ext } = resolveTarget();
  const binaryFile = os === "windows" ? `${BINARY_NAME}.exe` : BINARY_NAME;
  const archiveName = `${BINARY_NAME}_${os}_${arch}.${ext}`;
  const tag = `v${PKG_VERSION}`;
  const url = new URL(
    `https://github.com/${REPO}/releases/download/${tag}/${archiveName}`
  );

  console.log(`scholia install: downloading ${url}`);
  let archive;
  try {
    archive = await get(url);
  } catch (err) {
    fail(`could not download ${url}: ${err.message}`);
    return;
  }

  let binaryData;
  try {
    binaryData =
      ext === "zip"
        ? extractFromZip(archive, binaryFile)
        : extractFromTar(zlib.gunzipSync(archive), binaryFile);
  } catch (err) {
    fail(`could not extract ${binaryFile} from ${archiveName}: ${err.message}`);
    return;
  }
  if (!binaryData) fail(`${binaryFile} not found inside ${archiveName}`);

  const outDir = path.join(__dirname, "bin");
  fs.mkdirSync(outDir, { recursive: true });
  const outPath = path.join(outDir, binaryFile);
  fs.writeFileSync(outPath, binaryData, { mode: 0o755 });
  console.log(`scholia install: installed ${outPath}`);
}

main().catch((err) => fail(err.message));
