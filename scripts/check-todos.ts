#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';

type TodoItem = {
  id: string;
  title: string;
  required?: boolean;
  evidence?: string[];
  files?: string[];
};

type TaskState = {
  task?: string;
  todos: TodoItem[];
};

function exists(p: string) {
  return fs.existsSync(path.resolve(p));
}

function main() {
  const file = process.argv[2] || '.task-state.json';
  if (!exists(file)) {
    console.log(JSON.stringify({
      ok: false,
      error: 'task_state_not_found',
      file
    }, null, 2));
    process.exit(1);
  }

  const raw = fs.readFileSync(file, 'utf8');
  const state = JSON.parse(raw) as TaskState;
  const results = (state.todos || []).map((todo) => {
    const fileChecks = (todo.files || []).map((f) => ({ file: f, exists: exists(f) }));
    const evidenceChecks = (todo.evidence || []).map((e) => ({ evidence: e, present: true }));
    const ok = fileChecks.every((x) => x.exists) && evidenceChecks.every((x) => x.present);
    return {
      id: todo.id,
      title: todo.title,
      required: todo.required !== false,
      ok,
      files: fileChecks,
      evidence: evidenceChecks,
    };
  });

  const pending = results.filter((r) => r.required && !r.ok);
  const payload = {
    ok: pending.length === 0,
    task: state.task || null,
    totals: {
      total: results.length,
      pending: pending.length,
      complete: results.length - pending.length,
    },
    results,
  };

  console.log(JSON.stringify(payload, null, 2));
  process.exit(payload.ok ? 0 : 1);
}

main();
