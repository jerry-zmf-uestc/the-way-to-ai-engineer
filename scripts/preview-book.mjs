import { startBookDev } from './dev-book.mjs';

try {
  const session = startBookDev({ command: 'preview' });
  const result = await session.done;
  process.exitCode = result.code;
  if (result.error) process.stderr.write(`book preview: ${result.error.message}\n`);
} catch (error) {
  process.stderr.write(`book preview: ${error.message}\n`);
  process.exitCode = 1;
}
