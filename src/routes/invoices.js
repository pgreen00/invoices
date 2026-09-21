import Router from '@koa/router';

import { render } from '../render.js';
import { centsToInput, computeTotals } from '../money.js';
import { todayISO, workWeekOf, weekdaysFrom, addDays, isISODate } from '../dates.js';
import {
  buildInvoiceValues,
  businessSnapshot,
  clientSnapshot,
} from '../invoiceForm.js';
import {
  listInvoices,
  getInvoice,
  getLineItems,
  createInvoice,
  updateInvoice,
  deleteInvoice,
  nextInvoiceNumber,
  numberExists,
  invoiceStats,
  listClients,
  getClient,
  getSettings,
} from '../repo.js';

export const invoiceRoutes = new Router();

/* ------------------------------------------------------------------ */
/* Helpers                                                             */
/* ------------------------------------------------------------------ */

function toFormItem(item) {
  return {
    work_date: item.work_date ?? '',
    description: item.description ?? '',
    detail: item.detail ?? '',
    hours: item.hours ? String(item.hours) : '',
    rate: centsToInput(item.rate_cents ?? 0),
  };
}

function blankWeekItems(startISO, rateCents) {
  return weekdaysFrom(startISO).map((date) => ({
    work_date: date,
    description: '',
    detail: '',
    hours: '',
    rate: centsToInput(rateCents),
  }));
}

/** Everything the form template needs, for both new and edit. */
function formViewModel({ mode, action, invoice, items, errors = [], invoiceId = null }) {
  const clients = listClients();
  return {
    title: mode === 'edit' ? `Edit invoice ${invoice.number}` : 'New invoice',
    mode,
    action,
    invoiceId,
    invoice,
    items,
    errors,
    clients,
    clientOptions: clients.map((client) => ({
      id: client.id,
      name: client.name,
      default_project: client.default_project,
      default_contract: client.default_contract,
      default_po: client.default_po,
      default_rate: client.default_rate_cents == null ? '' : centsToInput(client.default_rate_cents),
      default_terms_days: client.default_terms_days,
    })),
    nextNumber: nextInvoiceNumber(),
    hasClients: clients.length > 0,
  };
}

function defaultsFor(client, settings) {
  const rateCents = client?.default_rate_cents ?? settings.default_rate_cents ?? 0;
  const termsDays = client?.default_terms_days ?? settings.default_terms_days ?? 15;
  return { rateCents, termsDays };
}

/* ------------------------------------------------------------------ */
/* List                                                                */
/* ------------------------------------------------------------------ */

invoiceRoutes.get('/', (ctx) => {
  render(ctx, 'invoices/list', {
    title: 'Invoices',
    invoices: listInvoices(),
    stats: invoiceStats(),
    hasClients: listClients().length > 0,
  });
});

/* ------------------------------------------------------------------ */
/* New                                                                 */
/* ------------------------------------------------------------------ */

invoiceRoutes.get('/invoices/new', (ctx) => {
  const settings = getSettings();
  const clients = listClients();
  const copyFrom = ctx.query.copy_from;

  if (copyFrom) {
    const source = getInvoice(copyFrom);
    if (!source) ctx.throw(404, `No invoice with id ${copyFrom}.`);

    // Roll the whole invoice forward one week, keeping the work descriptions
    // as a starting point for the new period.
    const sourceItems = getLineItems(source.id);
    const items = sourceItems.map((item) => ({
      ...toFormItem(item),
      work_date: item.work_date ? addDays(item.work_date, 7) : '',
      hours: '',
    }));

    const invoice = {
      number: '',
      client_id: source.client_id,
      project_name: source.project_name,
      contract_ref: source.contract_ref,
      po_number: source.po_number,
      invoice_date: todayISO(),
      period_start: source.period_start ? addDays(source.period_start, 7) : '',
      period_end: source.period_end ? addDays(source.period_end, 7) : '',
      terms_days: source.terms_days,
      expenses: '0.00',
      discount: '0.00',
      payment_instructions: source.payment_instructions,
      notes: source.notes,
      footer_message: source.footer_message,
    };

    render(
      ctx,
      'invoices/form',
      formViewModel({
        mode: 'new',
        action: '/invoices',
        invoice,
        items: items.length ? items : blankWeekItems(todayISO(), settings.default_rate_cents),
      })
    );
    return;
  }

  const client = ctx.query.client_id ? getClient(ctx.query.client_id) : clients[0];
  const { rateCents, termsDays } = defaultsFor(client, settings);
  const week = workWeekOf(isISODate(ctx.query.week) ? ctx.query.week : todayISO());

  const invoice = {
    number: '',
    client_id: client?.id ?? '',
    project_name: client?.default_project ?? '',
    contract_ref: client?.default_contract ?? '',
    po_number: client?.default_po ?? '',
    invoice_date: todayISO(),
    period_start: week.start,
    period_end: week.end,
    terms_days: termsDays,
    expenses: '0.00',
    discount: '0.00',
    payment_instructions: settings.payment_instructions,
    notes: settings.notes,
    footer_message: settings.footer_message,
  };

  render(
    ctx,
    'invoices/form',
    formViewModel({
      mode: 'new',
      action: '/invoices',
      invoice,
      items: blankWeekItems(week.start, rateCents),
    })
  );
});

/* ------------------------------------------------------------------ */
/* Create                                                              */
/* ------------------------------------------------------------------ */

invoiceRoutes.post('/invoices', (ctx) => {
  const body = ctx.request.body;
  const settings = getSettings();
  const client = body.client_id ? getClient(body.client_id) : null;

  const { values, items, errors } = buildInvoiceValues(body, {
    business: businessSnapshot(settings),
    client: clientSnapshot(client),
  });

  values.number = values.number || nextInvoiceNumber();
  if (numberExists(values.number)) {
    errors.push(`Invoice number ${values.number} is already in use.`);
  }

  if (errors.length > 0) {
    ctx.status = 422;
    render(
      ctx,
      'invoices/form',
      formViewModel({
        mode: 'new',
        action: '/invoices',
        invoice: { ...body, ...values, expenses: body.expenses, discount: body.discount },
        items: items.map(toFormItem),
        errors,
      })
    );
    return;
  }

  const id = createInvoice(values, items);
  ctx.redirect(`/invoices/${id}`);
});

/* ------------------------------------------------------------------ */
/* Edit / Update / Delete                                              */
/* ------------------------------------------------------------------ */

invoiceRoutes.get('/invoices/:id/edit', (ctx) => {
  const invoice = getInvoice(ctx.params.id);
  if (!invoice) ctx.throw(404, `No invoice with id ${ctx.params.id}.`);

  render(
    ctx,
    'invoices/form',
    formViewModel({
      mode: 'edit',
      action: `/invoices/${invoice.id}`,
      invoiceId: invoice.id,
      invoice: {
        ...invoice,
        expenses: centsToInput(invoice.expenses_cents),
        discount: centsToInput(invoice.discount_cents),
      },
      items: getLineItems(invoice.id).map(toFormItem),
    })
  );
});

invoiceRoutes.post('/invoices/:id', (ctx) => {
  const existing = getInvoice(ctx.params.id);
  if (!existing) ctx.throw(404, `No invoice with id ${ctx.params.id}.`);

  const body = ctx.request.body;
  const settings = getSettings();
  const submittedClientId = body.client_id ? Number(body.client_id) : null;

  // The business block stays frozen at whatever it was when the invoice was
  // generated. The client block is only re-snapshotted if you switch clients.
  const business = {
    business_name: existing.business_name,
    business_tagline: existing.business_tagline,
    business_address_line1: existing.business_address_line1,
    business_address_line2: existing.business_address_line2,
    business_city_state_zip: existing.business_city_state_zip,
    business_email: existing.business_email,
    business_phone: existing.business_phone,
    business_tax_id: existing.business_tax_id,
  };

  const client =
    submittedClientId && submittedClientId !== existing.client_id
      ? clientSnapshot(getClient(submittedClientId))
      : {
          client_name: existing.client_name,
          client_attn: existing.client_attn,
          client_address_line1: existing.client_address_line1,
          client_address_line2: existing.client_address_line2,
          client_city_state_zip: existing.client_city_state_zip,
          client_email: existing.client_email,
        };

  const { values, items, errors } = buildInvoiceValues(body, { business, client });

  values.number = values.number || existing.number;
  if (numberExists(values.number, existing.id)) {
    errors.push(`Invoice number ${values.number} is already in use.`);
  }

  if (errors.length > 0) {
    ctx.status = 422;
    render(
      ctx,
      'invoices/form',
      formViewModel({
        mode: 'edit',
        action: `/invoices/${existing.id}`,
        invoiceId: existing.id,
        invoice: { ...body, ...values, expenses: body.expenses, discount: body.discount },
        items: items.map(toFormItem),
        errors,
      })
    );
    return;
  }

  updateInvoice(existing.id, values, items);
  ctx.redirect(`/invoices/${existing.id}`);
});

invoiceRoutes.post('/invoices/:id/delete', (ctx) => {
  deleteInvoice(ctx.params.id);
  ctx.redirect('/');
});

/* ------------------------------------------------------------------ */
/* Print view                                                          */
/* ------------------------------------------------------------------ */

invoiceRoutes.get('/invoices/:id', (ctx) => {
  const invoice = getInvoice(ctx.params.id);
  if (!invoice) ctx.throw(404, `No invoice with id ${ctx.params.id}.`);

  const items = getLineItems(invoice.id);
  const totals = computeTotals(items, invoice.expenses_cents, invoice.discount_cents);

  render(
    ctx,
    'invoices/print',
    { title: `Invoice ${invoice.number}`, invoice, items, totals },
    { layout: false }
  );
});
