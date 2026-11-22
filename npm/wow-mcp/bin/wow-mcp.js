#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.
//
// npm launcher for the wow-mcp server. The server itself is a Go binary; npm installs exactly one
// platform package (wow-mcp-<os>-<arch>, an optional dependency) and this script runs the binary from it,
// passing every argument and the stdio streams through unchanged (MCP speaks JSON-RPC over stdio).
//
//   npx -y wow-mcp [server flags]              run the MCP server (see `--help`)
//   npx -y wow-mcp install-addon [options]     copy the bundled in-game addon into the game directory
"use strict";

const childProcess = require("child_process");
const fs = require("fs");
const path = require("path");

const PACKAGE_ROOT = path.join(__dirname, "..");

function platformPackageName() {
  return `wow-mcp-${process.platform}-${process.arch}`;
}

function binaryPath() {
  const exe = process.platform === "win32" ? "wow-mcp.exe" : "wow-mcp";
  const pkg = platformPackageName();
  try {
    return require.resolve(`${pkg}/bin/${exe}`);
  } catch (_) {
    process.stderr.write(
      `wow-mcp: no prebuilt server for ${process.platform}/${process.arch} (package ${pkg} is missing).\n` +
        "Supported: win32, darwin and linux on x64 and arm64. If your platform is one of them, reinstall without\n" +
        "--no-optional / --omit=optional. Otherwise build from source: https://github.com/Piekario/wow-mcp\n"
    );
    process.exit(1);
  }
}

// The Go server keeps its dump history (and goals.json next to it) beside its own binary by default. Under npm
// that binary lives in node_modules or the npx cache, which is replaced on every upgrade, so the launcher points
// the history at a stable per-user directory unless the caller chose one.
function userDataDir() {
  const home = require("os").homedir();
  if (process.platform === "win32") {
    return path.join(process.env.LOCALAPPDATA || path.join(home, "AppData", "Local"), "wow-mcp");
  }
  if (process.platform === "darwin") return path.join(home, "Library", "Application Support", "wow-mcp");
  return path.join(process.env.XDG_DATA_HOME || path.join(home, ".local", "share"), "wow-mcp");
}

function withDefaultHistoryDir(args) {
  const chosen = args.some((a) => a === "--history-dir" || a === "-history-dir" || /^--?history-dir=/.test(a));
  if (chosen) return args;
  return ["--history-dir", path.join(userDataDir(), "history"), ...args];
}

function runServer(args) {
  const child = childProcess.spawn(binaryPath(), withDefaultHistoryDir(args), { stdio: "inherit", windowsHide: true });
  for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
    process.on(signal, () => {
      if (!child.killed) child.kill(signal);
    });
  }
  child.on("error", (err) => {
    process.stderr.write(`wow-mcp: cannot start the server: ${err.message}\n`);
    process.exit(1);
  });
  child.on("exit", (code, signal) => {
    if (signal) process.kill(process.pid, signal);
    else process.exit(code === null ? 1 : code);
  });
}

// ---- install-addon ----------------------------------------------------------------------------------------

function bundledAddon() {
  const addonRoot = path.join(PACKAGE_ROOT, "addon");
  const dirs = fs.existsSync(addonRoot)
    ? fs.readdirSync(addonRoot).filter((d) => fs.existsSync(path.join(addonRoot, d, `${d}.toc`)))
    : [];
  if (dirs.length !== 1) throw new Error(`the package must bundle exactly one addon in ${addonRoot}`);
  return { name: dirs[0], dir: path.join(addonRoot, dirs[0]) };
}

function parseOptions(args) {
  const opts = { wowDir: process.env.WOW_DIR || "", client: process.env.WOW_CLIENT || "" };
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    const value = () => {
      if (i + 1 >= args.length) throw new Error(`${arg} needs a value`);
      return args[++i];
    };
    if (arg === "--wow-dir") opts.wowDir = value();
    else if (arg === "--client") opts.client = value();
    else if (arg === "-h" || arg === "--help") opts.help = true;
    else throw new Error(`unknown option ${arg}`);
  }
  return opts;
}

const CLIENT_FOLDER = /^_[a-z0-9_]+_$/;

// A game directory is the folder that holds the client folders (_retail_, _classic_era_, _forever_, ...).
// A path that points at a client folder itself is taken as its parent.
function gameDirOf(dir) {
  const clean = dir.replace(/[\\/]+$/, "");
  return CLIENT_FOLDER.test(path.basename(clean)) ? path.dirname(clean) : clean;
}

function clientFolders(wowDir) {
  try {
    return fs
      .readdirSync(wowDir, { withFileTypes: true })
      .filter((e) => e.isDirectory() && CLIENT_FOLDER.test(e.name))
      .map((e) => e.name)
      .sort();
  } catch (_) {
    return [];
  }
}

// Battle.net records the install path in the registry (InstallPath under Blizzard Entertainment, and the
// uninstall entry's InstallLocation). Read-only `reg query`; any failure just means "not found here".
function registryWowDirs() {
  if (process.platform !== "win32") return [];
  const queries = [
    ["HKLM\\SOFTWARE\\WOW6432Node\\Blizzard Entertainment", "/s", "/v", "InstallPath"],
    ["HKLM\\SOFTWARE\\Blizzard Entertainment", "/s", "/v", "InstallPath"],
    ["HKLM\\SOFTWARE\\WOW6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\World of Warcraft", "/v", "InstallLocation"],
  ];
  const dirs = [];
  for (const q of queries) {
    let out = "";
    try {
      out = childProcess.execFileSync("reg", ["query", ...q], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] });
    } catch (_) {
      continue;
    }
    for (const line of out.split(/\r?\n/)) {
      const m = line.match(/^\s*(?:InstallPath|InstallLocation)\s+REG_\w+\s+(.+?)\s*$/);
      if (m && /world of warcraft/i.test(m[1])) dirs.push(gameDirOf(m[1]));
    }
  }
  return dirs;
}

// Windows: the usual install folders on every drive, plus "World of Warcraft" one level below a drive root
// (e.g. D:\\Games\\World of Warcraft or D:\\Battlenet\\World of Warcraft). Only directory listings, nothing is opened.
function windowsDriveWowDirs() {
  const dirs = [];
  for (const letter of "CDEFGHIJKLMNOPQRSTUVWXYZ") {
    const root = `${letter}:\\`;
    if (!fs.existsSync(root)) continue;
    dirs.push(
      path.join(root, "Program Files (x86)", "World of Warcraft"),
      path.join(root, "Program Files", "World of Warcraft"),
      path.join(root, "World of Warcraft")
    );
    let entries = [];
    try {
      entries = fs.readdirSync(root, { withFileTypes: true });
    } catch (_) {
      continue;
    }
    for (const e of entries) {
      if (e.isDirectory() && !/^(windows|\$|system volume information)/i.test(e.name)) {
        dirs.push(path.join(root, e.name, "World of Warcraft"));
      }
    }
  }
  return dirs;
}

function defaultWowDirs() {
  if (process.platform === "win32") return windowsDriveWowDirs();
  if (process.platform === "darwin") return ["/Applications/World of Warcraft"];
  return [];
}

function resolveWowDir(opts) {
  if (opts.wowDir) return gameDirOf(opts.wowDir);
  const candidates = [...new Set([...registryWowDirs(), ...defaultWowDirs()])];
  const found = candidates.filter((d) => clientFolders(d).length > 0);
  if (found.length === 0) throw new Error("cannot find the game directory; pass --wow-dir <path> (or set WOW_DIR)");
  if (found.length > 1) {
    throw new Error(`found several game directories (${found.join(", ")}); pick one with --wow-dir`);
  }
  return found[0];
}

function resolveClient(wowDir, opts) {
  const clients = clientFolders(wowDir);
  if (opts.client) {
    if (!clients.includes(opts.client)) {
      throw new Error(`${path.join(wowDir, opts.client)} does not exist; clients found: ${clients.join(", ") || "none"}`);
    }
    return opts.client;
  }
  if (clients.length === 1) return clients[0];
  if (clients.length === 0) throw new Error(`${wowDir} has no client folder (_retail_, _classic_era_, ...); check --wow-dir`);
  throw new Error(`${wowDir} has several clients (${clients.join(", ")}); pick one with --client`);
}

function installAddon(args) {
  const opts = parseOptions(args);
  if (opts.help) {
    process.stdout.write(
      "usage: wow-mcp install-addon [--wow-dir <game directory>] [--client <client folder, e.g. _retail_>]\n" +
        "Copies the bundled addon into <game>/<client>/Interface/AddOns, keeping a backup of an older copy.\n" +
        "Without --wow-dir it uses WOW_DIR, then the path Battle.net stored in the Windows registry, then the usual\n" +
        "install folders (on Windows on every drive, also <drive>:\\<folder>\\World of Warcraft).\n" +
        "Without --client it uses WOW_CLIENT, or the only client folder in the game directory.\n" +
        "It writes nothing else in the game directory (never WTF).\n"
    );
    return;
  }
  const addon = bundledAddon();
  const wowDir = resolveWowDir(opts);
  const clientDir = path.join(wowDir, resolveClient(wowDir, opts));
  const addOns = path.join(clientDir, "Interface", "AddOns");
  const target = path.join(addOns, addon.name);
  fs.mkdirSync(addOns, { recursive: true });
  if (fs.existsSync(target)) {
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    const backup = path.join(clientDir, "Interface", `${addon.name}.backup-${stamp}`);
    fs.renameSync(target, backup);
    process.stdout.write(`previous copy moved to ${backup}\n`);
  }
  fs.cpSync(addon.dir, target, { recursive: true });
  process.stdout.write(
    `installed the ${addon.name} addon in ${target}\n` +
      "Restart the game completely (a /reload does not load new addon folders), then type /reload once in game.\n"
  );
}

function main(argv) {
  if (argv[0] === "install-addon") {
    try {
      installAddon(argv.slice(1));
    } catch (err) {
      process.stderr.write(`wow-mcp install-addon: ${err.message}\n`);
      process.exit(1);
    }
    return;
  }
  runServer(argv);
}

main(process.argv.slice(2));
