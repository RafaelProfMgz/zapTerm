// bridge.mjs — sobe o binário Go em modo headless (--ui=json) e troca NDJSON:
// eventos chegam pelo stdout do Go, comandos voltam pelo stdin. O Go é o
// cérebro; aqui só passa mensagem.
import {spawn} from 'node:child_process';
import {EventEmitter} from 'node:events';
import {createInterface} from 'node:readline';
import {existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');

function findBinary() {
  const candidates = [
    process.env.ZAPTERM_BIN,
    path.join(repoRoot, 'whatscli'),
    path.join(repoRoot, 'zapterm'),
    path.join(repoRoot, 'dist', 'ZapTerm-linux', 'zapterm'),
  ].filter(Boolean);
  for (const c of candidates) {
    if (existsSync(c)) return c;
  }
  throw new Error(
    'binário do ZapTerm não encontrado — rode "go build" na raiz do repo ' +
    'ou aponte ZAPTERM_BIN para o executável',
  );
}

export function createBridge() {
  const bin = findBinary();
  const proc = spawn(bin, ['--ui=json'], {stdio: ['pipe', 'pipe', 'inherit']});
  const events = new EventEmitter();
  events.setMaxListeners(50);

  const rl = createInterface({input: proc.stdout});
  rl.on('line', line => {
    let msg;
    try {
      msg = JSON.parse(line);
    } catch {
      msg = {type: 'text', text: line}; // linha solta vira log
    }
    events.emit(msg.type, msg);
  });
  proc.on('exit', code => events.emit('exit', {type: 'exit', code}));

  return {
    on: (type, fn) => events.on(type, fn),
    off: (type, fn) => events.off(type, fn),
    // send('select', [chatId]) → {"cmd":"select","params":["..."]}
    send: (cmd, params = []) => {
      try {
        proc.stdin.write(JSON.stringify({cmd, params}) + '\n');
      } catch {
        // processo já morreu; o evento 'exit' cuida do resto
      }
    },
    quit: () => {
      try {
        proc.stdin.write(JSON.stringify({cmd: 'quit'}) + '\n');
        proc.stdin.end();
      } catch {}
      setTimeout(() => proc.kill(), 1500).unref();
    },
  };
}
