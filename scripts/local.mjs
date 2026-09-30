import { spawn } from "node:child_process";
import { readFileSync, existsSync, copyFileSync } from "node:fs";
import { parseEnv } from "node:util";
import { fileURLToPath } from "node:url";
import { resolve, dirname } from "node:path";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const ssr = process.argv.includes("--ssr"),
  port = ssr ? 4173 : 5173,
  origin = `http://localhost:${port}`;
const env = { ...process.env };
for (const name of ["backend/.env", ".env", ".env.local"]) {
  const file = resolve(root, name);
  if (existsSync(file))
    Object.assign(env, parseEnv(readFileSync(file, "utf8")));
}
if (!env.DATABASE_URL) {
  const file = resolve(root, ".env.local");
  if (!existsSync(file))
    copyFileSync(resolve(root, ".env.local.example"), file);
  Object.assign(env, parseEnv(readFileSync(file, "utf8")));
}
// Docker service names are not reachable from a host process.
try {
  const url = new URL(env.DATABASE_URL);
  if (url.hostname === "postgres") throw new Error("Docker-only database host");
} catch {
  console.error(
    "DATABASE_URL in .env.local must point to your running local PostgreSQL, e.g. localhost:5432.",
  );
  process.exit(1);
}
Object.assign(env, {
  APP_ENV: "development",
  FRONTEND_BASE_URL: origin,
  PUBLIC_BASE_URL: origin,
  VITE_PUBLIC_BASE_URL: origin,
  PORT: "8080",
  API_ORIGIN: "http://localhost:8080",
  STORE_NAME: env.VITE_STORE_NAME || "گیاکو",
});
if (env.ZARINPAL_ENABLED === "true" && env.ZARINPAL_SANDBOX !== "true") {
  console.error(
    "Local launch requires sandbox payments. Use ZARINPAL_SANDBOX=true or disable payments.",
  );
  process.exit(1);
}
const windows = process.platform === "win32",
  children = new Set();
function launch(program, args, cwd, extraEnv = {}) {
  const child = spawn(program, args, {
    cwd: resolve(root, cwd),
    env: { ...env, ...extraEnv },
    stdio: "inherit",
    shell: windows && program === "npm",
    detached: !windows,
  });
  children.add(child);
  child.on("error", (err) => {
    console.error(
      `Cannot run ${program}: ${err.code}. Install Go 1.26 and Node 22.12+; run npm run setup.`,
    );
  });
  child.once("exit", () => children.delete(child));
  return child;
}
function run(program, args, cwd) {
  return new Promise((resolve, reject) => {
    const c = launch(program, args, cwd);
    c.once("error", reject);
    c.once("exit", (code) =>
      code === 0
        ? resolve()
        : reject(new Error(`${program} exited with ${code}`)),
    );
  });
}
let stopping = false;
function stop(code = 0) {
  if (stopping) return;
  stopping = true;
  process.exitCode = code;
  for (const c of children) {
    if (!c.pid) continue;
    if (windows)
      spawn("taskkill", ["/pid", String(c.pid), "/t", "/f"], {
        stdio: "ignore",
      });
    else {
      try {
        process.kill(-c.pid, "SIGTERM");
      } catch {}
    }
  }
  setTimeout(() => process.exit(code), 800).unref();
}
process.on("SIGINT", () => stop());
process.on("SIGTERM", () => stop());
try {
  await run("go", ["run", "./cmd/migrate"], "backend");
  if (ssr) await run("npm", ["run", "build"], "frontend");
  if (!existsSync(resolve(root, "frontend/node_modules"))) {
    throw new Error("Run npm run setup first.");
  }
  console.log(
    `\nStore address: ${origin}. PostgreSQL must remain running. Ctrl+C stops the app.\n`,
  );
  const backend = launch("go", ["run", "."], "backend");
  let ready = false;
  for (let i = 0; i < 120 && !stopping; i++) {
    try {
      const r = await fetch("http://localhost:8080/readyz", {
        signal: AbortSignal.timeout(500),
      });
      if (r.ok) {
        ready = true;
        break;
      }
    } catch {}
    await new Promise((r) => setTimeout(r, 250));
  }
  if (!ready)
    throw new Error("Backend did not become ready. Check its log above.");
  const front = ssr
    ? launch(process.execPath, ["server.mjs"], "frontend", {
        PORT: String(port),
      })
    : launch(
        "npm",
        [
          "run",
          "dev",
          "--",
          "--host",
          "127.0.0.1",
          "--port",
          String(port),
          "--strictPort",
        ],
        "frontend",
      );
  for (const c of [backend, front])
    c.once("exit", (code) => {
      if (!stopping) {
        console.error(`App stopped (code ${code}). Check the error above.`);
        stop(code || 1);
      }
    });
} catch (err) {
  console.error(err.message);
  console.error(
    "Check .env.local database credentials. No database or volume is deleted by this launcher.",
  );
  stop(1);
}
