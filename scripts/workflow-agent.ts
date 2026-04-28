#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { spawn } from 'node:child_process';
import YAML from 'yaml';

type Step = {
  name?: string;
  run?: string;
  env?: Record<string, string>;
  workingDirectory?: string;
};

type Job = {
  name?: string;
  env?: Record<string, string>;
  steps?: Step[];
  container?: string | { image?: string };
  defaults?: {
    run?: {
      shell?: string;
      'working-directory'?: string;
    };
  };
};

type Workflow = {
  jobs?: Record<string, Job>;
};

function readWorkflow(file: string): Workflow {
  return YAML.parse(fs.readFileSync(file, 'utf8')) as Workflow;
}

function emit(event: Record<string, unknown>) {
  process.stdout.write(JSON.stringify({ ts: new Date().toISOString(), ...event }) + os.EOL);
}

function mergeEnv(...parts: Array<Record<string, string> | undefined>) {
  return Object.assign({}, ...parts.filter(Boolean));
}

function resolveContainer(job: Job) {
  if (!job.container) return 'ubuntu:22.04';
  if (typeof job.container === 'string') return job.container;
  return job.container.image || 'ubuntu:22.04';
}

async function runShell(command: string, args: string[], env: NodeJS.ProcessEnv) {
  return await new Promise<number>((resolve, reject) => {
    const child = spawn(command, args, { env, stdio: ['ignore', 'pipe', 'pipe'] });

    child.stdout.on('data', (buf) => {
      for (const line of String(buf).split(/\r?\n/)) {
        if (line.trim()) emit({ type: 'stdout', line });
      }
    });

    child.stderr.on('data', (buf) => {
      for (const line of String(buf).split(/\r?\n/)) {
        if (line.trim()) emit({ type: 'stderr', line });
      }
    });

    child.on('close', (code) => resolve(code ?? 1));
    child.on('error', reject);
  });
}

async function runStep(opts: {
  image: string;
  cwd: string;
  shell: string;
  command: string;
  env: Record<string, string>;
}) {
  const dockerArgs = [
    'run', '--rm',
    '--network', 'none',
    '-v', `${opts.cwd}:/workspace`,
    '-w', '/workspace',
    ...Object.entries(opts.env).flatMap(([k, v]) => ['-e', `${k}=${v}`]),
    opts.image,
    opts.shell,
    '-lc',
    opts.command,
  ];

  return runShell('docker', dockerArgs, process.env);
}

async function runJob(jobId: string, job: Job, projectRoot: string) {
  const image = resolveContainer(job);
  const shell = job.defaults?.run?.shell || 'bash';
  const defaultWd = job.defaults?.run?.['working-directory'] || '.';
  const jobEnv = mergeEnv(job.env);

  emit({ type: 'job_started', job: jobId, image, shell });

  for (const [index, step] of (job.steps || []).entries()) {
    if (!step.run) continue;
    const stepName = step.name || `step_${index + 1}`;
    const stepEnv = mergeEnv(jobEnv, step.env);
    const cwd = path.resolve(projectRoot, step.workingDirectory || defaultWd);

    emit({ type: 'step_started', job: jobId, step: stepName, cwd });
    const exitCode = await runStep({
      image,
      cwd: projectRoot,
      shell,
      command: `cd ${JSON.stringify(path.relative(projectRoot, cwd) || '.')} && ${step.run}`,
      env: stepEnv,
    });
    emit({ type: 'step_finished', job: jobId, step: stepName, exitCode });

    if (exitCode !== 0) {
      emit({ type: 'job_finished', job: jobId, status: 'failed' });
      return 1;
    }
  }

  emit({ type: 'job_finished', job: jobId, status: 'success' });
  return 0;
}

async function main() {
  const workflowPath = process.argv[2] || '.github/workflows/ci.yml';
  const workflow = readWorkflow(workflowPath);
  const projectRoot = process.cwd();
  const targetJob = process.argv[3];
  const jobs = workflow.jobs || {};
  const selected = targetJob ? [[targetJob, jobs[targetJob]]] : Object.entries(jobs);

  if (!selected.length || selected.some(([, job]) => !job)) {
    emit({ type: 'error', message: 'job_not_found_or_empty' });
    process.exit(1);
  }

  let failures = 0;
  for (const [jobId, job] of selected as Array<[string, Job]>) {
    const code = await runJob(jobId, job, projectRoot);
    if (code !== 0) failures += 1;
  }

  emit({ type: 'workflow_finished', status: failures === 0 ? 'success' : 'failed', failures });
  process.exit(failures === 0 ? 0 : 1);
}

main().catch((error) => {
  emit({ type: 'error', message: error instanceof Error ? error.message : String(error) });
  process.exit(1);
});
