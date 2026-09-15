#!/usr/bin/env node
/**
 * Menjalankan tool Go milik apps/api (air, migrate, sqlc) tanpa bergantung pada PATH.
 *
 * Kenapa ada: `"dev:api": "cd apps/api && air"` mengandaikan `air` ada di PATH.
 * `go install` menaruh binarinya di GOPATH/bin, dan folder itu TIDAK otomatis
 * masuk PATH di Windows — jadi `npm run dev:api` menjawab "'air' is not
 * recognized as an internal or external command" padahal binarinya terpasang.
 * Menambahkannya ke ~/.bashrc pun tidak cukup: di Windows, npm menjalankan
 * script lewat cmd.exe, bukan bash, sehingga isi .bashrc tidak pernah dibaca.
 *
 * Script ini mencari sendiri binarinya (PATH → GOBIN → GOPATH/bin), dan bila
 * memang belum terpasang, memasangnya lewat `go install` pada versi yang
 * dipatok di bawah, lalu menjalankannya. Sekali kena, mesin baru mana pun
 * cukup `npm install && npm run dev:api`.
 *
 * Pemakaian:
 *   node scripts/go-tool.mjs [--cwd=<dir relatif repo>] <tool> [argumen...]
 *
 * Argumen boleh memuat placeholder `{{NAMA_ENV}}`, yang diganti dari
 * environment. Ini disengaja, bukan gaya: `$DATABASE_URL` diperluas oleh sh di
 * macOS/Linux tapi diteruskan mentah oleh cmd.exe di Windows, jadi satu baris
 * script npm tidak pernah berperilaku sama di dua tempat. `{{...}}` tidak
 * berarti apa-apa bagi kedua shell, jadi yang memperluasnya selalu script ini.
 */
import { spawn, spawnSync } from "node:child_process";
import { existsSync, statSync } from "node:fs";
import { delimiter, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const REPO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const IS_WINDOWS = process.platform === "win32";

/**
 * Versi dipatok, bukan @latest — `go install X@latest` pada mesin berbeda di
 * hari berbeda menghasilkan biner berbeda, dan itu bukan sesuatu yang pantas
 * terjadi diam-diam saat seseorang menjalankan `npm run dev:api`.
 *
 * `migrate` sengaja mengikuti versi pustaka golang-migrate di apps/api/go.mod
 * (v4.19.1): CLI dan pustaka yang dipakai `internal/migrator` membaca format
 * tabel `schema_migrations` yang sama, jadi keduanya wajib selangkah. Tag
 * `mysql` wajib — tanpanya biner migrate tidak punya driver database ini.
 */
const TOOLS = {
  air: { install: "github.com/air-verse/air@v1.67.1" },
  migrate: { install: "github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1", tags: "mysql" },
  sqlc: { install: "github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1" },
};

function fail(message) {
  console.error(`\n[go-tool] ${message}\n`);
  process.exit(1);
}

function goEnv(name) {
  const result = spawnSync("go", ["env", name], { encoding: "utf8" });
  if (result.error || result.status !== 0) return "";
  return (result.stdout ?? "").trim();
}

function isExecutableFile(path) {
  try {
    return statSync(path).isFile();
  } catch {
    return false;
  }
}

/** Urutan pencarian: PATH dulu (hormati versi yang sudah dipilih pengguna), baru lokasi default `go install`. */
function searchDirs() {
  const dirs = (process.env.PATH ?? "").split(delimiter).filter(Boolean);
  const gobin = goEnv("GOBIN");
  if (gobin) dirs.push(gobin);
  for (const gopath of goEnv("GOPATH").split(delimiter)) {
    if (gopath) dirs.push(join(gopath, "bin"));
  }
  return dirs;
}

function locate(tool) {
  const names = IS_WINDOWS ? [`${tool}.exe`, tool] : [tool];
  for (const dir of searchDirs()) {
    for (const name of names) {
      const candidate = join(dir, name);
      if (isExecutableFile(candidate)) return candidate;
    }
  }
  return null;
}

function install(tool, spec) {
  const args = ["install"];
  if (spec.tags) args.push("-tags", spec.tags);
  args.push(spec.install);
  console.error(`[go-tool] "${tool}" belum terpasang — menjalankan: go ${args.join(" ")}`);
  const result = spawnSync("go", args, { stdio: "inherit" });
  if (result.error) fail(`Gagal menjalankan "go". Pastikan Go terpasang dan ada di PATH.\n${result.error.message}`);
  return result.status === 0;
}

/** Mengganti {{NAMA}} dari environment; variabel yang belum diset digagalkan di sini, bukan diteruskan kosong ke tool. */
function expandPlaceholders(value) {
  return value.replace(/\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}/g, (_, name) => {
    const fromEnv = process.env[name];
    if (fromEnv === undefined || fromEnv === "") {
      fail(
        `Environment variable ${name} belum diset, padahal perintah ini membutuhkannya.\n` +
          `Set dulu, misalnya:  export ${name}="..."   (Git Bash)  /  $env:${name}="..."   (PowerShell)`
      );
    }
    return fromEnv;
  });
}

function main() {
  let argv = process.argv.slice(2);

  let cwd = REPO_ROOT;
  if (argv[0]?.startsWith("--cwd=")) {
    cwd = resolve(REPO_ROOT, argv[0].slice("--cwd=".length));
    argv = argv.slice(1);
  }
  if (!existsSync(cwd)) fail(`Direktori kerja tidak ditemukan: ${cwd}`);

  const tool = argv[0];
  if (!tool) fail("Tool belum disebutkan. Pemakaian: node scripts/go-tool.mjs [--cwd=<dir>] <tool> [argumen...]");

  const spec = TOOLS[tool];
  if (!spec) fail(`Tool "${tool}" tidak dikenal. Yang terdaftar: ${Object.keys(TOOLS).join(", ")}`);

  let binary = locate(tool);
  if (!binary) {
    if (!install(tool, spec)) fail(`Gagal memasang "${tool}". Jalankan manual: go install ${spec.install}`);
    binary = locate(tool);
    // Terpasang tapi tetap tidak ketemu berarti GOBIN/GOPATH menunjuk ke tempat
    // lain dari yang kita geledah — sebut folder yang diperiksa, jangan cuma
    // bilang "tidak ditemukan".
    if (!binary) {
      fail(
        `"${tool}" terpasang tapi binarinya tidak ditemukan di:\n  ${searchDirs().join("\n  ")}\n` +
          `Cek: go env GOBIN && go env GOPATH`
      );
    }
  }

  const args = argv.slice(1).map(expandPlaceholders);
  const child = spawn(binary, args, { cwd, stdio: "inherit" });

  // Ctrl+C sudah sampai ke anak lewat process group. Induk sengaja tidak ikut
  // mati supaya exit code anak (air/migrate) yang diteruskan ke npm, bukan
  // kode terminasi induk.
  const ignore = () => {};
  process.on("SIGINT", ignore);
  process.on("SIGTERM", ignore);

  child.on("error", (err) => fail(`Gagal menjalankan ${binary}\n${err.message}`));
  child.on("exit", (code, signal) => process.exit(signal ? 1 : (code ?? 0)));
}

main();
