import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const migrationsDir = path.join(root, 'services/api-gateway/migrations');
const migrations = fs.readdirSync(migrationsDir).filter((name) => /^\d{6}_.*\.sql$/.test(name)).sort();
if (!migrations.length) throw new Error('No versioned migrations found');
const latest = migrations.at(-1).match(/^(\d{6})_/)[1];
const targets = [
  path.join(root, 'infra/docker-compose/.env.example'),
  path.join(root, 'infra/docker-compose/docker-compose.yml'),
];
const mismatches = targets.filter((file) => !fs.readFileSync(file, 'utf8').includes(`000${latest.slice(3)}`));
export function check() {
  if (mismatches.length) throw new Error(`Schema version ${latest} is not synchronized in: ${mismatches.join(', ')}`);
  console.log(`Schema version ${latest} is synchronized.`);
}
export function sync() {
  for (const file of targets) {
    const source = fs.readFileSync(file, 'utf8');
    const updated = source
      .replace(/(EDUGRADE_SCHEMA_VERSION=)\d{6}/, `$1${latest}`)
      .replace(/(SCHEMA_VERSION: \$\{EDUGRADE_SCHEMA_VERSION:-)\d{6}(\})/, `$1${latest}$2`);
    fs.writeFileSync(file, updated);
  }
  console.log(`Synchronized schema metadata to ${latest}.`);
}
