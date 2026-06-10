#!/usr/bin/env node
/**
 * Build the docs site and package the standalone tree into a tar.gz
 * artifact suitable for shipyard deploy.
 *
 * Output: ./dist/shipyard-web-release.tar.gz
 *
 * Called by the pre_upload hook in shipyard.yaml at the repo root.
 */
import { execSync } from 'node:child_process';
import { existsSync, mkdirSync, cpSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(__dirname, '..');
const repoRoot = resolve(webRoot, '..');
const standalone = resolve(webRoot, '.next/standalone');
const distDir = resolve(repoRoot, 'dist');
const archive = resolve(distDir, 'shipyard-web-release.tar.gz');

function run(cmd, opts = {}) {
  console.log(`▸ ${cmd}`);
  execSync(cmd, { stdio: 'inherit', cwd: opts.cwd || webRoot });
}

console.log('━━━ packaging shipyard-web ━━━');

run('npm run build');

const standaloneStatic = resolve(standalone, '.next/static');
const standalonePublic = resolve(standalone, 'public');
mkdirSync(dirname(standaloneStatic), { recursive: true });
cpSync(resolve(webRoot, '.next/static'), standaloneStatic, { recursive: true });
if (existsSync(resolve(webRoot, 'public'))) {
  cpSync(resolve(webRoot, 'public'), standalonePublic, { recursive: true });
}

mkdirSync(distDir, { recursive: true });
if (existsSync(archive)) rmSync(archive);
run(`tar -czf ${archive} -C ${standalone} .`);

console.log(`━━━ packaged: ${archive} ━━━`);
