import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { FunctionCode } from '../../src/errors';

/**
 * `FunctionCode` names what an invoke fails with, and the retry loop keys on
 * one of them (`FUNCTION_TIMEOUT`). A value that drifted from the gateway's
 * constant would silently turn that check off, so each is read back out of the
 * Go source.
 */

const goSource = resolve(__dirname, '../../../core/pkg/httputil/rpc_error.go');
const haveGo = existsSync(goSource);
const go = haveGo ? readFileSync(goSource, 'utf8') : '';

describe('function error codes match the gateway', () => {
  it('finds the gateway source', () => {
    expect(haveGo, `${goSource} — this test cannot check parity`).toBe(true);
  });

  it.skipIf(!haveGo)('claims no code the gateway does not send', () => {
    const declared = new Set([...go.matchAll(/ErrCode\w+\s+RPCErrorCode\s+=\s+"([A-Z_]+)"/g)].map((m) => m[1]));
    const invented = Object.values(FunctionCode).filter((code) => !declared.has(code));
    expect(invented, 'codes the SDK names that no gateway constant declares').toEqual([]);
  });
});
