/* Progressive enhancement only: every form works without this file,
   the server recomputes all totals and the due date on save. */

const money = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
});

/* ---------- Confirm before destructive posts ---------- */
document.addEventListener('submit', (event) => {
  const message = event.target.dataset?.confirm;
  if (message && !window.confirm(message)) event.preventDefault();
});

/* ---------- Invoice form ---------- */
const form = document.getElementById('invoice-form');
if (form) initInvoiceForm(form);

function initInvoiceForm(form) {
  const body = document.getElementById('items-body');
  const rowTemplate = document.getElementById('item-row-template');
  const expenses = document.getElementById('expenses');
  const discount = document.getElementById('discount');
  const invoiceDate = document.getElementById('invoice-date');
  const termsDays = document.getElementById('terms-days');
  const dueDate = document.getElementById('due-date');
  const periodStart = document.getElementById('period-start');
  const periodEnd = document.getElementById('period-end');
  const clientSelect = document.getElementById('client-select');

  const num = (input) => {
    const value = parseFloat(input?.value ?? '');
    return Number.isFinite(value) ? value : 0;
  };

  /* ----- Totals ----- */
  function recalculate() {
    let hours = 0;
    let subtotal = 0;

    for (const row of body.querySelectorAll('.item-row')) {
      const rowHours = num(row.querySelector('[name="item_hours"]'));
      const rowRate = num(row.querySelector('[name="item_rate"]'));
      const amount = Math.round(rowHours * rowRate * 100) / 100;

      hours += rowHours;
      subtotal += amount;
      row.querySelector('.amount-cell').textContent = money.format(amount);
    }

    const total = subtotal + num(expenses) - num(discount);
    document.getElementById('total-hours').textContent = hours.toFixed(2);
    document.getElementById('subtotal').textContent = money.format(subtotal);
    document.getElementById('grand-total').textContent = money.format(total);
  }

  /* ----- Due date ----- */
  function updateDueDate() {
    const parsed = parseISO(invoiceDate.value);
    const days = parseInt(termsDays.value, 10);

    if (!parsed || !Number.isFinite(days)) {
      dueDate.textContent = '—';
      return;
    }
    parsed.setDate(parsed.getDate() + days);
    dueDate.textContent = parsed.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  }

  /* ----- Rows ----- */
  function addRow(date = '', rate = '') {
    const row = rowTemplate.content.firstElementChild.cloneNode(true);
    row.querySelector('[name="item_date"]').value = date;
    row.querySelector('[name="item_rate"]').value = rate || lastRate();
    body.appendChild(row);
    return row;
  }

  function lastRate() {
    const rates = [...body.querySelectorAll('[name="item_rate"]')];
    return rates.length ? rates[rates.length - 1].value : '';
  }

  document.getElementById('add-row').addEventListener('click', () => {
    const row = addRow();
    row.querySelector('[name="item_description"]').focus();
    recalculate();
  });

  body.addEventListener('click', (event) => {
    if (!event.target.classList.contains('remove-row')) return;
    const rows = body.querySelectorAll('.item-row');
    if (rows.length === 1) {
      // Keep one row around so the form is never empty.
      for (const input of rows[0].querySelectorAll('input')) {
        if (input.name !== 'item_rate') input.value = '';
      }
    } else {
      event.target.closest('.item-row').remove();
    }
    recalculate();
  });

  /* ----- Fill Mon–Fri ----- */
  document.getElementById('fill-week').addEventListener('click', () => {
    const start = parseISO(periodStart.value) || new Date();
    const monday = new Date(start);
    const dow = monday.getDay();
    monday.setDate(monday.getDate() + (dow === 0 ? -6 : 1 - dow));

    const dates = [0, 1, 2, 3, 4].map((offset) => {
      const date = new Date(monday);
      date.setDate(date.getDate() + offset);
      return toISO(date);
    });

    periodStart.value = dates[0];
    periodEnd.value = dates[4];

    dates.forEach((date, index) => {
      const row = body.querySelectorAll('.item-row')[index] ?? addRow();
      row.querySelector('[name="item_date"]').value = date;
    });
    recalculate();
  });

  /* ----- Client defaults ----- */
  if (clientSelect) {
    const defaults = readJSON('client-defaults') ?? [];
    const byId = new Map(defaults.map((client) => [String(client.id), client]));
    let previous = byId.get(clientSelect.value) ?? null;

    clientSelect.addEventListener('change', () => {
      const next = byId.get(clientSelect.value);
      if (!next) return;

      // Only replace a value the user has not customised: either blank, or
      // still holding the previously selected client's default.
      const apply = (element, nextValue, previousValue) => {
        if (!element || nextValue === undefined || nextValue === null || nextValue === '') return;
        if (element.value === '' || element.value === (previousValue ?? '')) {
          element.value = nextValue;
        }
      };

      apply(document.getElementById('project-name'), next.default_project, previous?.default_project);
      apply(document.getElementById('contract-ref'), next.default_contract, previous?.default_contract);
      apply(document.getElementById('po-number'), next.default_po, previous?.default_po);
      apply(termsDays, next.default_terms_days, previous?.default_terms_days);

      if (next.default_rate) {
        for (const row of body.querySelectorAll('.item-row')) {
          const hours = row.querySelector('[name="item_hours"]');
          const rate = row.querySelector('[name="item_rate"]');
          // Leave rows the user has already filled in alone.
          if (!hours.value) rate.value = next.default_rate;
        }
      }

      previous = next;
      updateDueDate();
      recalculate();
    });
  }

  form.addEventListener('input', () => {
    recalculate();
    updateDueDate();
  });

  recalculate();
  updateDueDate();
}

/* ---------- Local-date helpers (never parse ISO as UTC) ---------- */
function parseISO(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value ?? '')) return null;
  const [year, month, day] = value.split('-').map(Number);
  const date = new Date(year, month - 1, day);
  return Number.isNaN(date.getTime()) ? null : date;
}

function toISO(date) {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

function readJSON(id) {
  const element = document.getElementById(id);
  if (!element) return null;
  try {
    return JSON.parse(element.textContent);
  } catch {
    return null;
  }
}
