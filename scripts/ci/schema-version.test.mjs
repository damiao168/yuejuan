import assert from 'node:assert/strict';
import test from 'node:test';
import { readComposeSchemaVersion, readEnvSchemaVersion } from './schema-version.mjs';

test('parses only the deployment schema fields', () => {
  assert.equal(readEnvSchemaVersion('OTHER=000999\nEDUGRADE_SCHEMA_VERSION=000117\n'), '000117');
  assert.equal(readComposeSchemaVersion('OTHER: 000999\nSCHEMA_VERSION: ${EDUGRADE_SCHEMA_VERSION:-000117}\n'), '000117');
});

test('rejects missing or malformed schema fields', () => {
  assert.equal(readEnvSchemaVersion('EDUGRADE_SCHEMA_VERSION=117\n'), undefined);
  assert.equal(readComposeSchemaVersion('SCHEMA_VERSION: 000117\n'), undefined);
});
