import Router from '@koa/router';

import { render } from '../render.js';
import { centsToInput } from '../money.js';
import { parseClientBody } from '../invoiceForm.js';
import {
  listClients,
  getClient,
  createClient,
  updateClient,
  deleteClient,
} from '../repo.js';

export const clientRoutes = new Router({ prefix: '/clients' });

function toFormClient(client) {
  return {
    ...client,
    default_rate: client.default_rate_cents == null ? '' : centsToInput(client.default_rate_cents),
  };
}

clientRoutes.get('/', (ctx) => {
  render(ctx, 'clients/list', {
    title: 'Clients',
    clients: listClients({ includeArchived: true }),
  });
});

clientRoutes.get('/new', (ctx) => {
  render(ctx, 'clients/form', {
    title: 'New client',
    mode: 'new',
    action: '/clients',
    client: { archived: 0 },
    errors: [],
  });
});

clientRoutes.post('/', (ctx) => {
  const { values, errors } = parseClientBody(ctx.request.body);

  if (errors.length > 0) {
    ctx.status = 422;
    render(ctx, 'clients/form', {
      title: 'New client',
      mode: 'new',
      action: '/clients',
      client: { ...ctx.request.body, ...values },
      errors,
    });
    return;
  }

  createClient(values);
  ctx.redirect('/clients');
});

clientRoutes.get('/:id/edit', (ctx) => {
  const client = getClient(ctx.params.id);
  if (!client) ctx.throw(404, `No client with id ${ctx.params.id}.`);

  render(ctx, 'clients/form', {
    title: `Edit ${client.name}`,
    mode: 'edit',
    action: `/clients/${client.id}`,
    client: toFormClient(client),
    errors: [],
  });
});

clientRoutes.post('/:id', (ctx) => {
  const client = getClient(ctx.params.id);
  if (!client) ctx.throw(404, `No client with id ${ctx.params.id}.`);

  const { values, errors } = parseClientBody(ctx.request.body);

  if (errors.length > 0) {
    ctx.status = 422;
    render(ctx, 'clients/form', {
      title: `Edit ${client.name}`,
      mode: 'edit',
      action: `/clients/${client.id}`,
      client: { ...ctx.request.body, ...values, id: client.id },
      errors,
    });
    return;
  }

  updateClient(client.id, values);
  ctx.redirect('/clients');
});

clientRoutes.post('/:id/delete', (ctx) => {
  // Invoices keep their snapshotted billing details; the FK is set to NULL.
  deleteClient(ctx.params.id);
  ctx.redirect('/clients');
});
