// mouse.mjs — suporte a mouse via protocolo SGR do terminal (CSI ? 1000/1006).
// O Ink não tem mouse nativo: habilitamos o rastreio no terminal e filtramos
// as sequências do stdin ANTES de chegarem ao Ink — senão o parser de teclado
// as trataria como texto e elas iriam parar dentro do TextInput.
import {PassThrough} from 'node:stream';
import {EventEmitter} from 'node:events';
import {StringDecoder} from 'node:string_decoder';

const ENABLE = '\x1b[?1000h\x1b[?1006h';
const DISABLE = '\x1b[?1006l\x1b[?1000l';

export function enableMouse(stdout = process.stdout) {
  if (stdout.isTTY) stdout.write(ENABLE);
}

export function disableMouse(stdout = process.stdout) {
  if (stdout.isTTY) stdout.write(DISABLE);
}

// evento SGR completo: ESC [ < código ; coluna ; linha (M=press|m=release)
const SEQ = /\x1b\[<(\d+);(\d+);(\d+)([Mm])/g;
// prefixo de sequência cortada no fim do chunk — fica no buffer até completar
const PARTIAL = /\x1b(?:\[(?:<[\d;]{0,16})?)?$/;

// createMouseStdin embrulha o stdin real: devolve um stream para passar ao
// render() do Ink (sem as sequências de mouse) e um emitter com os eventos
// já decodificados: {type: 'click'|'wheel', x, y, dy} — x/y em colunas/linhas
// 1-based do terminal.
export function createMouseStdin(real = process.stdin) {
  const events = new EventEmitter();
  const stdin = new PassThrough();
  // o Ink consulta/chama estes no stdin que recebe; delega ao stdin real
  stdin.isTTY = real.isTTY;
  stdin.setRawMode = mode => {
    if (real.isTTY) real.setRawMode(mode);
    return stdin;
  };
  stdin.ref = () => {
    real.ref?.();
    return stdin;
  };
  stdin.unref = () => {
    real.unref?.();
    return stdin;
  };

  const decoder = new StringDecoder('utf8');
  let buf = '';
  real.on('data', chunk => {
    buf += decoder.write(chunk);
    let out = '';
    let last = 0;
    SEQ.lastIndex = 0;
    let m;
    while ((m = SEQ.exec(buf))) {
      out += buf.slice(last, m.index);
      last = SEQ.lastIndex;
      const code = Number(m[1]);
      const x = Number(m[2]);
      const y = Number(m[3]);
      if (code === 64 || code === 65) {
        // roda: 64 sobe, 65 desce (só no press; wheel não tem release)
        events.emit('mouse', {type: 'wheel', x, y, dy: code === 64 ? -1 : 1});
      } else if (code === 0 && m[4] === 'M') {
        // botão esquerdo sem modificadores, no press
        events.emit('mouse', {type: 'click', x, y, dy: 0});
      }
    }
    buf = buf.slice(last);
    const partial = PARTIAL.exec(buf);
    if (partial) {
      out += buf.slice(0, partial.index);
      buf = buf.slice(partial.index);
    } else {
      out += buf;
      buf = '';
    }
    if (out) stdin.write(out);
  });
  real.on('end', () => stdin.end());

  return {stdin, events};
}
