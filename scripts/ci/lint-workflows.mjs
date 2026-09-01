import { spawnSync } from 'node:child_process';

const result = spawnSync(
  'docker',
  ['run', '--rm', '-v', `${process.cwd()}:/repo`, '-w', '/repo', 'rhysd/actionlint:1.7.7'],
  { stdio: 'inherit', shell: process.platform === 'win32' },
);

if (result.error) {
  console.error(`Unable to start actionlint: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status ?? 1);
