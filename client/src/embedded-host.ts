import type { EmbeddedIssue, HydrationOptions, TypedSnapshot } from './embedded-contract';
import { parseCanonicalPath } from './embedded-values';

/** Host persistence context; row IDs are stable identities, not array indexes. */
export interface SnapshotContext { definitionRevision: string; rows: Record<string, string[]> }
export interface SnapshotEnvelope extends SnapshotContext { version: 'formgen.host-snapshot/v1'; snapshot: TypedSnapshot }
interface SnapshotController {
  snapshot(): TypedSnapshot;
  restore(snapshot: TypedSnapshot, options?: HydrationOptions): EmbeddedIssue[];
}
function rowSignature(rows: unknown): string | null {
  if (!rows || typeof rows !== 'object' || Array.isArray(rows)) return null;
  const pairs: [string, string[]][] = [];
  for (const [path, ids] of Object.entries(rows)) {
    if (!parseCanonicalPath(path) || !Array.isArray(ids) || ids.some(id => typeof id !== 'string' || !id.trim()) || new Set(ids).size !== ids.length) return null;
    pairs.push([path, ids]);
  }
  return JSON.stringify(pairs.sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0));
}
function validContext(context: SnapshotContext): boolean {
  return typeof context?.definitionRevision === 'string' && !!context.definitionRevision.trim() && rowSignature(context.rows) !== null;
}
export function captureSnapshotEnvelope(controller: SnapshotController, context: SnapshotContext): SnapshotEnvelope {
  if (!validContext(context)) throw new Error('A definition revision and valid row mapping are required');
  return JSON.parse(JSON.stringify({ version: 'formgen.host-snapshot/v1', definitionRevision: context.definitionRevision, rows: context.rows, snapshot: controller.snapshot() })) as SnapshotEnvelope;
}
/** Reject context mismatches before calling restore. Legacy snapshots require explicit host migration. */
export function restoreSnapshotEnvelope(controller: SnapshotController, envelope: unknown, context: SnapshotContext, options?: HydrationOptions): EmbeddedIssue[] {
  const saved = envelope as SnapshotEnvelope | null;
  if (!validContext(context) || !saved || saved.version !== 'formgen.host-snapshot/v1' || !validContext(saved) ||
      saved.definitionRevision !== context.definitionRevision || rowSignature(saved.rows) !== rowSignature(context.rows)) {
    return [{ code: 'snapshotContextMismatch', message: 'Snapshot definition revision or row mapping differs; explicit migration is required' }];
  }
  return controller.restore(saved.snapshot, options);
}
/** Generate today's fixed reference; removed IDs cannot silently become another row. */
export function fixedRowFieldPath(rows: SnapshotContext['rows'], arrayPath: string, rowID: string, fieldPath: string): string {
  if (rowSignature(rows) === null || !Object.prototype.hasOwnProperty.call(rows,arrayPath)) throw new Error('Invalid row mapping');
  const index = rows[arrayPath].indexOf(rowID);
  const path = `${arrayPath}[${index}].${fieldPath}`;
  if (index < 0 || !parseCanonicalPath(fieldPath) || !parseCanonicalPath(path)) throw new Error('Unknown row or invalid field path');
  return path;
}
