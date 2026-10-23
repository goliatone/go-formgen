import { fileURLToPath } from 'node:url';
import { resolve, dirname } from 'node:path';
import { writeManifest, checkManifest } from './artifacts.mjs';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
await (process.argv.includes('--check') ? checkManifest(root) : writeManifest(root));
console.log('[formgen] artifact manifest verified');
