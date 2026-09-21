-- Single-row table holding your business details and form defaults.
CREATE TABLE IF NOT EXISTS settings (
  id                   INTEGER PRIMARY KEY CHECK (id = 1),
  business_name        TEXT    NOT NULL DEFAULT '',
  business_tagline     TEXT    NOT NULL DEFAULT '',
  address_line1        TEXT    NOT NULL DEFAULT '',
  address_line2        TEXT    NOT NULL DEFAULT '',
  city_state_zip       TEXT    NOT NULL DEFAULT '',
  email                TEXT    NOT NULL DEFAULT '',
  phone                TEXT    NOT NULL DEFAULT '',
  tax_id               TEXT    NOT NULL DEFAULT '',
  default_rate_cents   INTEGER NOT NULL DEFAULT 0,
  default_terms_days   INTEGER NOT NULL DEFAULT 15,
  payment_instructions TEXT    NOT NULL DEFAULT '',
  notes                TEXT    NOT NULL DEFAULT '',
  footer_message       TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS clients (
  id                 INTEGER PRIMARY KEY,
  name               TEXT    NOT NULL,
  attn               TEXT    NOT NULL DEFAULT '',
  address_line1      TEXT    NOT NULL DEFAULT '',
  address_line2      TEXT    NOT NULL DEFAULT '',
  city_state_zip     TEXT    NOT NULL DEFAULT '',
  email              TEXT    NOT NULL DEFAULT '',
  default_project    TEXT    NOT NULL DEFAULT '',
  default_contract   TEXT    NOT NULL DEFAULT '',
  default_po         TEXT    NOT NULL DEFAULT '',
  default_rate_cents INTEGER,
  default_terms_days INTEGER,
  archived           INTEGER NOT NULL DEFAULT 0,
  created_at         TEXT    NOT NULL
);

-- Business and client details are snapshotted onto each invoice so that
-- editing settings or a client never rewrites history.
CREATE TABLE IF NOT EXISTS invoices (
  id                        INTEGER PRIMARY KEY,
  number                    TEXT    NOT NULL UNIQUE,
  client_id                 INTEGER REFERENCES clients(id) ON DELETE SET NULL,

  business_name             TEXT    NOT NULL DEFAULT '',
  business_tagline          TEXT    NOT NULL DEFAULT '',
  business_address_line1    TEXT    NOT NULL DEFAULT '',
  business_address_line2    TEXT    NOT NULL DEFAULT '',
  business_city_state_zip   TEXT    NOT NULL DEFAULT '',
  business_email            TEXT    NOT NULL DEFAULT '',
  business_phone            TEXT    NOT NULL DEFAULT '',
  business_tax_id           TEXT    NOT NULL DEFAULT '',

  client_name               TEXT    NOT NULL DEFAULT '',
  client_attn               TEXT    NOT NULL DEFAULT '',
  client_address_line1      TEXT    NOT NULL DEFAULT '',
  client_address_line2      TEXT    NOT NULL DEFAULT '',
  client_city_state_zip     TEXT    NOT NULL DEFAULT '',
  client_email              TEXT    NOT NULL DEFAULT '',

  project_name              TEXT    NOT NULL DEFAULT '',
  contract_ref              TEXT    NOT NULL DEFAULT '',
  po_number                 TEXT    NOT NULL DEFAULT '',

  invoice_date              TEXT    NOT NULL,
  period_start              TEXT    NOT NULL DEFAULT '',
  period_end                TEXT    NOT NULL DEFAULT '',
  terms_days                INTEGER NOT NULL DEFAULT 15,
  due_date                  TEXT    NOT NULL DEFAULT '',

  expenses_cents            INTEGER NOT NULL DEFAULT 0,
  discount_cents            INTEGER NOT NULL DEFAULT 0,

  payment_instructions      TEXT    NOT NULL DEFAULT '',
  notes                     TEXT    NOT NULL DEFAULT '',
  footer_message            TEXT    NOT NULL DEFAULT '',

  generated_at              TEXT    NOT NULL,
  updated_at                TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS line_items (
  id           INTEGER PRIMARY KEY,
  invoice_id   INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
  position     INTEGER NOT NULL DEFAULT 0,
  work_date    TEXT    NOT NULL DEFAULT '',
  description  TEXT    NOT NULL DEFAULT '',
  detail       TEXT    NOT NULL DEFAULT '',
  hours        REAL    NOT NULL DEFAULT 0,
  rate_cents   INTEGER NOT NULL DEFAULT 0,
  amount_cents INTEGER GENERATED ALWAYS AS (CAST(ROUND(hours * rate_cents) AS INTEGER)) STORED
);

CREATE INDEX IF NOT EXISTS idx_line_items_invoice ON line_items(invoice_id, position);
CREATE INDEX IF NOT EXISTS idx_invoices_date       ON invoices(invoice_date DESC);
CREATE INDEX IF NOT EXISTS idx_clients_name        ON clients(archived, name);
