import { DatabaseSync } from 'node:sqlite';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

class ConfigError extends Error {}

/**
 * Turns DATABASE_URL into an absolute filesystem path.
 * Accepts a plain path, a `~`-prefixed path, or a file: URL.
 */
export function resolveDatabasePath(raw) {
  const value = (raw ?? '').trim();

  if (!value) {
    throw new ConfigError(
      'DATABASE_URL is not set.\n\n' +
        'Create a .env file next to package.json containing the path to your\n' +
        'SQLite file, for example:\n\n' +
        '  DATABASE_URL=/Users/you/Somewhere/invoices.db\n\n' +
        'See .env.example for the supported formats.'
    );
  }

  let filePath;
  if (/^file:\/\//i.test(value)) {
    filePath = fileURLToPath(value);
  } else if (/^file:/i.test(value)) {
    filePath = value.slice('file:'.length);
  } else if (/^[a-z][a-z0-9+.-]*:\/\//i.test(value)) {
    throw new ConfigError(
      `DATABASE_URL must be a local file path or file: URL, got "${value}".\n` +
        'SQLite has no network protocol; point this at a file on disk.'
    );
  } else {
    filePath = value;
  }

  if (filePath === '~' || filePath.startsWith('~/')) {
    filePath = path.join(os.homedir(), filePath.slice(1));
  }

  filePath = path.resolve(filePath);

  // Deliberately not creating the directory: a typo should fail loudly
  // rather than silently scatter empty databases around the filesystem.
  const dir = path.dirname(filePath);
  if (!fs.existsSync(dir)) {
    throw new ConfigError(
      `The directory for DATABASE_URL does not exist:\n  ${dir}\n\n` +
        'Create it first, or correct the path in .env.'
    );
  }

  return filePath;
}

function applyPragmas(db) {
  // DELETE journalling keeps the database a single self-contained file at
  // rest, which is far friendlier to folder-syncing and file-copy backups
  // than WAL's -wal/-shm sidecars.
  db.exec('PRAGMA journal_mode = DELETE');
  db.exec('PRAGMA foreign_keys = ON');
  db.exec('PRAGMA busy_timeout = 5000');
  db.exec('PRAGMA synchronous = FULL');
}

function seed(db) {
  const hasSettings = db.prepare('SELECT COUNT(*) AS n FROM settings').get().n > 0;
  if (!hasSettings) {
    db.prepare(
      `INSERT INTO settings (
         id, business_name, business_tagline, address_line1, address_line2,
         city_state_zip, email, phone, tax_id, default_rate_cents,
         default_terms_days, payment_instructions, notes, footer_message
       ) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
    ).run(
      'Ravenline Software LLC',
      'Custom Software Development',
      '1420 Beacon Street',
      'Suite 300',
      'Austin, TX 78701',
      'billing@ravenline.dev',
      '(512) 555-0142',
      'EIN 87-1234567',
      16500,
      15,
      [
        'ACH / Wire — Lone Star Bank, N.A.',
        'Account Name: Ravenline Software LLC',
        'Routing: 111000025 · Account: 000123456789',
      ].join('\n'),
      [
        'Payment due within 15 days of the invoice date.',
        'Late balances accrue 1.5% interest per month.',
        'Hours are billed in 15-minute increments; detailed time logs available on request.',
      ].join('\n'),
      'Thank you for your business.'
    );
  }

  const hasClients = db.prepare('SELECT COUNT(*) AS n FROM clients').get().n > 0;
  if (!hasClients) {
    db.prepare(
      `INSERT INTO clients (
         name, attn, address_line1, address_line2, city_state_zip, email,
         default_project, default_contract, default_po,
         default_rate_cents, default_terms_days, created_at
       ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
    ).run(
      'Northwind Retail Group, Inc.',
      'Dana Whitfield, VP Engineering',
      '98 Harbor Point Drive',
      'Floor 12',
      'Seattle, WA 98104',
      'ap@northwindretail.com',
      'Order Management Platform — Phase 2',
      'MSA dated 2024-01-15',
      'NW-PO-4471',
      16500,
      15,
      new Date().toISOString()
    );
  }
}

let db;

export function openDatabase() {
  if (db) return db;

  const filePath = resolveDatabasePath(process.env.DATABASE_URL);

  try {
    db = new DatabaseSync(filePath);
  } catch (err) {
    throw new ConfigError(`Could not open the database at ${filePath}\n  ${err.message}`);
  }

  applyPragmas(db);
  db.exec(fs.readFileSync(path.join(HERE, 'schema.sql'), 'utf8'));
  seed(db);

  db.databasePath = filePath;
  return db;
}

export function getDb() {
  if (!db) throw new Error('Database not opened yet; call openDatabase() first.');
  return db;
}

/** Runs `fn` inside a transaction, rolling back on any throw. */
export function transaction(fn) {
  const handle = getDb();
  handle.exec('BEGIN');
  try {
    const result = fn(handle);
    handle.exec('COMMIT');
    return result;
  } catch (err) {
    try {
      handle.exec('ROLLBACK');
    } catch {
      /* the original error is the interesting one */
    }
    throw err;
  }
}

export { ConfigError };
