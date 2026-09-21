import { openDatabase, ConfigError } from './db.js';
import { createApp } from './app.js';

let db;
try {
  db = openDatabase();
} catch (err) {
  if (err instanceof ConfigError) {
    console.error(`\nConfiguration error\n-------------------\n${err.message}\n`);
    process.exit(1);
  }
  throw err;
}

const port = Number(process.env.PORT) || 3000;
const host = process.env.HOST || '127.0.0.1';

createApp().listen(port, host, () => {
  console.log(`Invoices  →  http://${host}:${port}`);
  console.log(`Database  →  ${db.databasePath}`);
});

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => {
    db.close();
    process.exit(0);
  });
}
