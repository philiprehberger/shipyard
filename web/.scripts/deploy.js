#!/usr/bin/env node
/**
 * Legacy rsync deploy. Cloned from philiprehberger-nextjs/.scripts/deploy.js.
 *
 * Once Shipyard v0.0.1 ships, we replace this with `shipyard deploy
 * shipyard.yaml` from the repo root. Until then, this script bootstraps
 * the first version of the docs site so the binary has a home.
 */
import { execSync } from 'node:child_process';
import { existsSync, mkdirSync, cpSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(__dirname, '..');

// Read .env from web/
const envPath = resolve(webRoot, '.env');
if (existsSync(envPath)) {
  const lines = (await import('node:fs')).readFileSync(envPath, 'utf8').split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq < 0) continue;
    const key = trimmed.slice(0, eq).trim();
    let val = trimmed.slice(eq + 1).trim();
    if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
      val = val.slice(1, -1);
    }
    if (!process.env[key]) process.env[key] = val;
  }
}

const required = ['SERVER_HOST', 'SERVER_USERNAME', 'SERVER_PRIVATE_KEY', 'SERVER_DEST_PATH', 'SERVER_PM2_PROCESS'];
const missing = required.filter((k) => !process.env[k]);
if (missing.length) {
  console.error(`Missing env vars: ${missing.join(', ')}\nSee web/.env.example for the required keys.`);
  process.exit(1);
}

const host = process.env.SERVER_HOST;
const user = process.env.SERVER_USERNAME;
const key = process.env.SERVER_PRIVATE_KEY.replace(/^~\//, `${process.env.HOME}/`);
const dest = process.env.SERVER_DEST_PATH;
const pm2name = process.env.SERVER_PM2_PROCESS;

function run(cmd, opts = {}) {
  console.log(`▸ ${cmd}`);
  execSync(cmd, { stdio: 'inherit', cwd: webRoot, ...opts });
}

console.log('━━━ Shipyard docs site — legacy rsync deploy ━━━');

// 1. Build
run('npm run build');

// 2. Copy static + public into .next/standalone so the rsync target is one tree.
const standaloneRoot = resolve(webRoot, '.next/standalone');
const standaloneStatic = resolve(standaloneRoot, '.next/static');
const standalonePublic = resolve(standaloneRoot, 'public');
mkdirSync(dirname(standaloneStatic), { recursive: true });
cpSync(resolve(webRoot, '.next/static'), standaloneStatic, { recursive: true });
if (existsSync(resolve(webRoot, 'public'))) {
  cpSync(resolve(webRoot, 'public'), standalonePublic, { recursive: true });
}

// 3. Rsync up. --delete keeps the destination tidy across deploys.
const sshOpts = `ssh -i ${key} -o StrictHostKeyChecking=accept-new`;
run(`rsync -az --delete --exclude=.env -e "${sshOpts}" ${standaloneRoot}/ ${user}@${host}:${dest}/`);

// 4. Graceful PM2 reload via login shell (nvm-resolved pm2 binary).
run(`${sshOpts} ${user}@${host} 'bash -lc "pm2 reload ${pm2name}"'`);

console.log('━━━ deploy complete ━━━');
