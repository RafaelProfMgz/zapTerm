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
// Quando empacotado (bun --compile), import.meta.url aponta p/ um caminho
// virtual; o núcleo Go vem ao lado do executável. process.execPath dá a pasta
// real do binário tanto no modo empacotado quanto rodando via "node".
const execDir = path.dirname(process.execPath);

function findBinary() {
  const candidates = [
    process.env.ZAPTERM_BIN,
    // núcleo empacotado ao lado do launcher Ink (nomes Linux e Windows)
    path.join(execDir, 'zapterm-core'),
    path.join(execDir, 'zapterm-core.exe'),
    path.join(execDir, 'whatscli'),
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

  // eventos chegados antes de a UI anexar os listeners (núcleo rápido) ficam
  // num buffer e são reemitidos no start(), chamado depois do primeiro render
  let started = false;
  const pending = [];
  const emit = msg => {
    if (!started) {
      pending.push(msg);
      return;
    }
    // prefixo "ev:" porque "error" é especial no EventEmitter (sem listener,
    // emitir "error" derruba o processo)
    events.emit('ev:' + msg.type, msg);
  };

  const rl = createInterface({input: proc.stdout});
  rl.on('line', line => {
    let msg;
    try {
      msg = JSON.parse(line);
    } catch {
      msg = {type: 'text', text: line}; // linha solta vira log
    }
    emit(msg);
  });
  proc.on('exit', code => emit({type: 'exit', code}));

  return {
    bin, // caminho do núcleo Go — exibido na tela CONFIG
    on: (type, fn) => events.on('ev:' + type, fn),
    off: (type, fn) => events.off('ev:' + type, fn),
    start: () => {
      if (started) return;
      started = true;
      for (const msg of pending.splice(0)) events.emit('ev:' + msg.type, msg);
    },
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
