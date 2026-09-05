import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const checks = [
  {
    name: 'runtime migration exists',
    file: 'services/api-gateway/migrations/000023_story051_agent_worker_runtime.sql',
    includes: ['agent_worker_task', 'agent_worker_task_attempt', 'agent_worker_heartbeat', 'uq_agent_worker_task_idempotency'],
  },
  {
    name: 'runtime state machine exists',
    file: 'services/api-gateway/internal/workerruntime/types.go',
    includes: ['StatusQueued', 'StatusDeadLetter', 'IdempotencyKey', 'LeaseToken'],
  },
  {
    name: 'postgres runtime exists',
    file: 'services/api-gateway/internal/workerruntime/store_postgres.go',
    includes: ['FOR UPDATE SKIP LOCKED', 'func (s *PostgresStore) Heartbeat', 'func (s *PostgresStore) Metrics'],
  },
  {
    name: 'runtime routes exist',
    file: 'services/api-gateway/internal/server/server.go',
    includes: [
      'POST /api/v1/internal/worker/tasks/claim',
      'POST /api/v1/internal/worker/tasks/{taskId}/heartbeat',
      'POST /api/v1/internal/worker/tasks/{taskId}/complete',
      'GET /api/v1/internal/worker/metrics',
    ],
  },
  {
    name: 'image quality handler uses the transactional runtime adapter',
    file: 'services/api-gateway/internal/imagequality/handlers.go',
    includes: ['CreateRunsWithTasks', 'SubmitResultCommand', 'LeaseRun', 'workerruntime.StatusRunning'],
  },
  {
    name: 'image quality runtime coordinator is atomic',
    file: 'services/api-gateway/internal/imagequality/coordinator_postgres.go',
    includes: ['image-quality-run:', 'CreateTaskInTx', 'CompleteTaskInTx', 'FailTaskInTx'],
  },
  {
    name: 'ocr worker uses runtime',
    file: 'services/ocr-worker/ocr_worker/runner.py',
    includes: ['claim_tasks', 'heartbeat_task', 'runtime_task_id', 'runtime_lease_token'],
  },
  {
    name: 'image quality worker reports retryable failure',
    file: 'services/image-quality-worker/image_quality/runner.py',
    includes: ['submit_failure', 'retryable_error', 'image_quality_processing_failed'],
  },
  {
    name: 'story document records approval evidence',
    file: 'docs/stories/STORY-051-agent-worker-runtime.md',
    includes: ['## Implementation', '## Implementation Review', '## Approval'],
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
  console.error('STORY-051 worker runtime check failed:');
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log('STORY-051 worker runtime check passed.');
