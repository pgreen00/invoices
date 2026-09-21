import fs from 'node:fs';
import path from 'node:path';
import querystring from 'node:querystring';

const MAX_BODY_BYTES = 2 * 1024 * 1024;

/**
 * Minimal urlencoded form parser. Line items are submitted as parallel
 * arrays (item_hours, item_rate, ...), so plain querystring parsing is
 * enough and we avoid pulling in a body-parser dependency.
 */
export function formBody() {
  return async (ctx, next) => {
    if (ctx.method === 'POST' || ctx.method === 'PUT' || ctx.method === 'PATCH') {
      ctx.request.body = querystring.parse(await readBody(ctx.req));
    }
    await next();
  };
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const chunks = [];

    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        reject(Object.assign(new Error('Request body too large'), { status: 413 }));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

const MIME = {
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.webmanifest': 'application/manifest+json; charset=utf-8',
};

function sendFile(ctx, file) {
  let stat;
  try {
    stat = fs.statSync(file);
  } catch {
    return false;
  }
  if (!stat.isFile()) return false;

  ctx.type = MIME[path.extname(file)] ?? 'application/octet-stream';
  ctx.set('Cache-Control', 'no-cache');
  ctx.length = stat.size;
  ctx.body = fs.createReadStream(file);
  return true;
}

export function staticFiles(prefix, root) {
  const dir = path.resolve(root);

  return async (ctx, next) => {
    if (ctx.method !== 'GET' && ctx.method !== 'HEAD') return next();
    if (!ctx.path.startsWith(prefix)) return next();

    const relative = decodeURIComponent(ctx.path.slice(prefix.length)).replace(/^\/+/, '');
    if (!relative) return next();

    const file = path.resolve(dir, relative);
    if (file !== dir && !file.startsWith(dir + path.sep)) return next();

    if (!sendFile(ctx, file)) return next();
  };
}

/**
 * Serves a fixed allow-list of files from the site root, for the handful of
 * things that browsers expect there: /favicon.ico, /manifest.webmanifest and
 * friends.
 */
export function rootFiles(root, names) {
  const dir = path.resolve(root);
  const allowed = new Set(names.map((name) => `/${name}`));

  return async (ctx, next) => {
    if (ctx.method !== 'GET' && ctx.method !== 'HEAD') return next();
    if (!allowed.has(ctx.path)) return next();

    if (!sendFile(ctx, path.join(dir, ctx.path.slice(1)))) return next();
  };
}

/** Renders thrown errors as a readable page instead of a bare stack trace. */
export function errorHandler() {
  return async (ctx, next) => {
    try {
      await next();
    } catch (err) {
      ctx.status = err.status || 500;
      ctx.type = 'html';
      ctx.body = `<!doctype html><meta charset="utf-8">
<title>${ctx.status} — Invoices</title>
<style>
  body{font:14px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
       max-width:44rem;margin:4rem auto;padding:0 1.5rem;color:#1c1f23}
  h1{font-size:1.25rem;margin:0 0 .75rem}
  pre{background:#f1f2f4;padding:1rem;border-radius:6px;overflow:auto;white-space:pre-wrap}
  a{color:#2563eb}
</style>
<h1>${ctx.status === 404 ? 'Not found' : 'Something went wrong'}</h1>
<pre>${escapeHtml(err.message ?? String(err))}</pre>
<p><a href="/">Back to invoices</a></p>`;

      if (ctx.status >= 500) ctx.app.emit('error', err, ctx);
    }
  };
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}
