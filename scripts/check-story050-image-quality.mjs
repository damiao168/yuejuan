import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();

const checks = [
  {
    name: 'image quality worker package exists',
    file: 'services/image-quality-worker/pyproject.toml',
    includes: ['edugrade-image-quality-worker'],
  },
  {
    name: 'image quality engine exists',
    file: 'services/image-quality-worker/image_quality/engine.py',
    includes: ['analyze_and_normalize', 'sharpness_score', 'normalized_png'],
  },
  {
    name: 'image quality runner exists',
    file: 'services/image-quality-worker/image_quality/runner.py',
    includes: ['claim_jobs', 'request_normalized_asset', 'submit_result'],
  },
  {
    name: 'story050 migration exists',
    file: 'services/api-gateway/migrations/000022_story050_image_quality_run.sql',
    includes: ['submission_page_quality_run', 'latest_quality_run_id', 'lease_token'],
  },
  {
    name: 'api routes exist',
    file: 'services/api-gateway/internal/server/server.go',
    includes: [
      'POST /api/v1/submissions/{id}/run-quality-check',
      'POST /api/v1/internal/image-quality/jobs/claim',
      'POST /api/v1/internal/image-quality/runs/{runId}/result',
    ],
  },
  {
    name: 'image quality worker Dockerfile exists',
    file: 'services/image-quality-worker/Dockerfile',
    includes: ['CMD ["python", "-m", "image_quality"]'],
  },
  {
    name: 'compose quality profile exists',
    file: 'infra/docker-compose/docker-compose.yml',
    includes: ['image-quality-worker:', 'profiles: ["quality"]', 'EDUGRADE_IMAGE_QUALITY_WORKER_ID'],
  },
  {
    name: 'package script exists',
    file: 'package.json',
    includes: ['check:story050', 'check-story050-image-quality.mjs'],
  },
];

const failures = [];
for (const check of checks) {
  const absolute = path.join(root, check.file);
  if (!fs.existsSync(absolute)) {
    failures.push(`${check.name}: missing ${check.file}`);
    continue;
  }
  const text = fs.readFileSync(absolute, 'utf8');
  for (const expected of check.includes) {
    if (!text.includes(expected)) {
      failures.push(`${check.name}: ${check.file} missing ${expected}`);
    }
  }
}

if (failures.length > 0) {
  console.error('STORY-050 image quality check failed:');
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log('STORY-050 image quality check passed.');
