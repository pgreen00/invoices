import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Koa from 'koa';

import { formBody, staticFiles, errorHandler } from './middleware.js';
import { invoiceRoutes } from './routes/invoices.js';
import { clientRoutes } from './routes/clients.js';
import { settingsRoutes } from './routes/settings.js';

const ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');

/** Builds the Koa app. The database must already be open. */
export function createApp() {
  const app = new Koa();

  app.use(errorHandler());
  app.use(staticFiles('/assets', path.join(ROOT, 'public')));
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
