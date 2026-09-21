import Router from '@koa/router';

import { render } from '../render.js';
import { centsToInput } from '../money.js';
import { parseSettingsBody } from '../invoiceForm.js';
import { getSettings, saveSettings } from '../repo.js';

export const settingsRoutes = new Router({ prefix: '/settings' });

function toForm(settings) {
  return { ...settings, default_rate: centsToInput(settings.default_rate_cents) };
}

settingsRoutes.get('/', (ctx) => {
  render(ctx, 'settings/form', {
    title: 'Settings',
    settings: toForm(getSettings()),
    errors: [],
    saved: ctx.query.saved === '1',
  });
});

settingsRoutes.post('/', (ctx) => {
  const { values, errors } = parseSettingsBody(ctx.request.body);

  if (errors.length > 0) {
    ctx.status = 422;
    render(ctx, 'settings/form', {
      title: 'Settings',
      settings: { ...ctx.request.body, ...values },
      errors,
      saved: false,
    });
    return;
  }

  saveSettings(values);
  ctx.redirect('/settings?saved=1');
});
