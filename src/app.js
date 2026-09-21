import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Koa from 'koa';

import { formBody, staticFiles, rootFiles, errorHandler } from './middleware.js';
import { invoiceRoutes } from './routes/invoices.js';
import { clientRoutes } from './routes/clients.js';
import { settingsRoutes } from './routes/settings.js';

const ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');

/** Builds the Koa app. The database must already be open. */
export function createApp() {
  const app = new Koa();

  const publicDir = path.join(ROOT, 'public');

  app.use(errorHandler());
  app.use(staticFiles('/assets', publicDir));
  app.use(
    rootFiles(publicDir, [
      'favicon.ico',
      'icon.svg',
      'icon-192.png',
      'icon-512.png',
      'apple-touch-icon.png',
      'manifest.webmanifest',
    ])
  );
  app.use(formBody());

  for (const router of [invoiceRoutes, clientRoutes, settingsRoutes]) {
    app.use(router.routes()).use(router.allowedMethods());
  }

  app.on('error', (err) => {
    if (err.status && err.status < 500) return;
    console.error('Unhandled error:', err);
  });

  return app;
}
