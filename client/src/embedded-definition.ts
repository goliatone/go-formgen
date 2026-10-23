import type { DefinitionDiagnostic, EmbeddedField } from './embedded-contract';
import { resolveEmbeddedModel, type EmbeddedModelInput } from './embedded-values';

/** Conservative ASCII pattern grammar. No shorthand classes, groups, dot, negation or flags. */
export function isPortablePattern(pattern: string): boolean {
  if (typeof pattern !== "string" || pattern.trim()!==pattern || pattern.length > 1024 || [...pattern].some(c => c.charCodeAt(0) < 32 || c.charCodeAt(0) > 126)) return false;
  const body = pattern.replace(/^\^/, '').replace(/\$$/, '');
  // Tokenize rather than accepting arbitrary engine syntax, then ask the engine for validity.
  const atom = /(?:\\[\\^$.*+?()[\]{}|\-]|\[[A-Za-z0-9 _.-]+\]|[A-Za-z0-9 _:@,\/=-])(?:[?*+]|\{(?:0|[1-9]\d{0,2})(?:,(?:0|[1-9]\d{0,2}))?\})?/gy;
  let at = 0;
  while (at < body.length) {
    atom.lastIndex = at;
    const match = atom.exec(body);
    if (!match) return false;
    at = atom.lastIndex;
  }
  try { new RegExp(pattern); return true; } catch { return false; }
}

/** Compare a bounded JSON decimal lexeme without first rounding it to binary64. */
function isPortableBound(text: string): boolean {
  if (typeof text !== 'string' || text.length > 128 || !/^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$/.test(text)) return false;
  const [coefficient, exponentText = '0'] = text.replace(/^-/, '').split(/[eE]/);
  const exponent = Number(exponentText);
  if (Math.abs(exponent) > 308) return false;
  const [whole, fraction = ''] = coefficient.split('.');
  const digits = (whole + fraction).replace(/^0+/, '');
  if (!digits) return true;
  const decimalPlaces = fraction.length - exponent;
  const integerLength = digits.length - decimalPlaces;
  const limit = '9007199254740991';
  if (integerLength !== limit.length) return integerLength < limit.length;
  const integer = digits.slice(0, integerLength).padEnd(integerLength, '0');
  if (integer !== limit) return integer < limit;
  return !/[1-9]/.test(digits.slice(integerLength));
}

/** Opt-in preflight; run lossless Go JSON preflight before parsing definitions in JS. */
export function preflightEmbeddedDefinition(input: EmbeddedModelInput): DefinitionDiagnostic[] {
  const out: DefinitionDiagnostic[] = [];
  const add = (code: string, path: string, message: string) => out.push({ code, path, message });
  const checkNumbers = (value: unknown, path: string): void => {
    if (typeof value === 'number' && (!Number.isFinite(value) || Math.abs(value) > Number.MAX_SAFE_INTEGER)) add('unsafeDefinitionNumber', path, 'Browser definition number exceeds the portable range');
    else if (Array.isArray(value)) value.forEach((v, i) => checkNumbers(v, `${path}/${i}`));
    else if (value && typeof value === 'object') Object.entries(value).forEach(([k,v]) => checkNumbers(v, `${path}/${k.replace(/~/g,'~0').replace(/\//g,'~1')}`));
  };
  const walk = (field: EmbeddedField, path: string): void => {
    checkNumbers(field.default, `${path}/default`);
    checkNumbers(field.enum, `${path}/enum`);
    checkNumbers(field.options, `${path}/options`);
    if (field.type === 'integer' || field.type === 'array' && field.items?.type === 'integer') {
      for (const [name, values] of [['default', field.default === undefined ? [] : field.type==='array' && Array.isArray(field.default) ? field.default : [field.default]], ['enum', field.enum ?? []]] as const) {
        values.forEach((value, i) => { if (value !== null && (typeof value !== 'number' || !Number.isInteger(value))) add('invalidDefinitionInteger', `${path}/${name}${name==='enum'?`/${i}`:''}`, 'Integer definitions require integral values'); });
      }
    }
    if (field.type === 'integer' || field.type === 'array' && field.items?.type==='integer') field.options?.forEach((option,i)=>{if(option.value!==null && (typeof option.value!=='number'||!Number.isInteger(option.value)))add('invalidDefinitionInteger',`${path}/options/${i}/value`,'Integer options require integral numeric values');});
    field.validations?.forEach((rule,i) => {
      const loc = `${path}/validations/${i}/params`;
      if (rule.kind === 'pattern' && !isPortablePattern(rule.params?.pattern ?? '')) add('unsupportedPattern', `${loc}/pattern`, 'Pattern is outside the portable ASCII grammar');
      if (['min','max'].includes(rule.kind) && !isPortableBound(rule.params?.value ?? '')) add('unsafeDefinitionNumber', `${loc}/value`, 'Browser bound exceeds the portable range');
    });
    field.nested?.forEach((child,i) => walk(child, `${path}/nested/${i}`));
    if (field.items) walk(field.items, `${path}/items`);
  };
  const model = resolveEmbeddedModel(input);
  model.fields.forEach((field,i) => walk(field, `#/fields/${i}`));
  checkNumbers(model.conditions, '#/conditions');
  return out;
}
