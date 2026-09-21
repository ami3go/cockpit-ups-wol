import { beforeEach, describe, expect, it, vi } from 'vitest';
import { applyConfig, getConfigStatus, getHealth, getLogs, getPlan, rollbackConfig, validateConfig } from './api';

type SpawnCall = { command: string[]; options?: CockpitSpawnOptions; input?: string };

function installCockpit(response = '{}') {
  const calls: SpawnCall[] = [];
  const spawn = vi.fn((command: string[], options?: CockpitSpawnOptions) => {
    const call: SpawnCall = { command, options };
    calls.push(call);
    const promise = Promise.resolve(response) as CockpitProcess;
    promise.input = (data: string | null | undefined) => { call.input = data ?? undefined; };
    return promise;
  });
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { cockpit: { spawn } },
  });
  return { calls };
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('Cockpit command privilege boundary', () => {
  it('uses try privilege for read-only commands', async () => {
    const { calls } = installCockpit('{"ok":true,"result":{"state":"HEALTHY","checked_at":"now","results":[],"generation":1}}');
    await getHealth();
    await getConfigStatus();
    await getPlan();
    await getLogs();
    expect(calls.map((c) => c.options?.superuser)).toEqual(['try', 'try', 'try', 'try']);
  });

  it('keeps mutations privileged', async () => {
    const { calls } = installCockpit('{}');
    await applyConfig('mode: dry-run\n');
    await rollbackConfig('revision-1');
    expect(calls.map((c) => c.options?.superuser)).toEqual(['require', 'require']);
  });

  it('passes candidate config on stdin while validation stays read-only', async () => {
    const { calls } = installCockpit('{"valid":true,"plan":{}}');
    await validateConfig('mode: dry-run\n');
    expect(calls).toHaveLength(1);
    expect(calls[0].command[calls[0].command.length - 1]).toBe('config-validate');
    expect(calls[0].options?.superuser).toBe('try');
    expect(calls[0].input).toBe('mode: dry-run\n');
  });
});
