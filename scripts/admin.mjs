import { spawn } from 'node:child_process'
import { readFileSync, existsSync } from 'node:fs'
import { parseEnv } from 'node:util'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..'),
  env = { ...process.env }
for (const name of ['backend/.env', '.env', '.env.local']) {
  const file = resolve(root, name)
  if (existsSync(file)) Object.assign(env, parseEnv(readFileSync(file, 'utf8')))
}
if (!env.DATABASE_URL) {
  console.error('Set DATABASE_URL in .env.local first.')
  process.exit(1)
}
const child = spawn('go', ['run', './cmd/admin', ...process.argv.slice(2)], {
  cwd: resolve(root, 'backend'),
  env,
  stdio: 'inherit',
})
child.on('error', () => {
  console.error('Go is required.')
  process.exitCode = 1
})
child.on('exit', (code) => {
  process.exitCode = code || 0
})
