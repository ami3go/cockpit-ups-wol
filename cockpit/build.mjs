#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { build } from 'esbuild';

const outdir = 'dist';
fs.rmSync(outdir, { recursive: true, force: true });
fs.mkdirSync(outdir, { recursive: true });
await build({
  entryPoints: ['src/index.tsx'],
  outdir,
  bundle: true,
  minify: true,
  sourcemap: true,
  target: ['es2020'],
  loader: { '.woff': 'file', '.woff2': 'file', '.ttf': 'file', '.svg': 'file', '.png': 'file', '.jpg': 'file' },
  define: { 'process.env.NODE_ENV': '"production"' },
});
for (const name of ['index.html', 'manifest.json']) fs.copyFileSync(path.join('src', name), path.join(outdir, name));
