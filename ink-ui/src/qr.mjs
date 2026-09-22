// qr.mjs — tela de leitura do QR code de login. O núcleo Go manda a matriz
// ('1' módulo escuro, '0' claro, já com a zona silenciosa), e aqui ela é
// desenhada em meia altura para caber no terminal e continuar escaneável.
import React from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

// preto e branco de verdade (sem cor do tema): a câmera do celular precisa do
// contraste real entre os módulos
const DARK = 'black';
const LIGHT = 'white';

// qrRuns transforma a matriz em linhas de meia altura: cada '▀' pinta o módulo
// de cima na cor da fonte e o de baixo no fundo, então duas linhas de módulos
// ocupam uma linha do terminal. Módulos iguais em sequência viram um trecho só,
// para não criar um elemento por módulo.
export function qrRuns(matrix) {
  const rows = [];
  for (let y = 0; y < matrix.length; y += 2) {
    const top = matrix[y];
    const bottom = matrix[y + 1] ?? '0'.repeat(top.length); // linha ímpar: zona clara
    const runs = [];
    for (let x = 0; x < top.length; x++) {
      const fg = top[x] === '1' ? DARK : LIGHT;
      const bg = bottom[x] === '1' ? DARK : LIGHT;
      const last = runs[runs.length - 1];
      if (last && last.fg === fg && last.bg === bg) last.len += 1;
      else runs.push({fg, bg, len: 1});
    }
    rows.push(runs);
  }
  return rows;
}

// qrFits diz se o desenho cabe no espaço disponível — em terminal pequeno vale
// mais mandar o usuário abrir o PNG do que mostrar um QR cortado.
export function qrFits(matrix, width, height) {
  if (!matrix || matrix.length === 0) return false;
  return matrix[0].length <= width && qrHeight(matrix) <= height;
}

// qrHeight: linhas de terminal que o desenho ocupa (dois módulos por linha).
export function qrHeight(matrix) {
  return Math.ceil((matrix?.length || 0) / 2);
}

// CHROME é o que a tela usa além do QR: título, mensagem e rodapé. Um código
// de pareamento real tem ~277 caracteres = matriz 65x65, ou seja 33 linhas de
// terminal — o enquadramento precisa ser apertado para caber em janelas
// comuns, e os respiros só entram quando sobra espaço.
export const CHROME = 3;

export function qrLayout(matrix, width, height) {
  const fits = qrFits(matrix, width - 2, height - CHROME);
  return {fits, roomy: fits && qrFits(matrix, width - 2, height - CHROME - 3)};
}

function Code({matrix}) {
  const rows = qrRuns(matrix);
  return h(Box, {flexDirection: 'column'},
    ...rows.map((runs, i) => h(Text, {key: i},
      ...runs.map((r, j) => h(Text, {key: j, color: r.fg, backgroundColor: r.bg}, '▀'.repeat(r.len))),
    )),
  );
}

export default function QRScreen({qr, height, width, accountLabel}) {
  const matrix = qr?.matrix || [];
  const {fits, roomy} = qrLayout(matrix, width, height);
  // tudo em linhas/colunas do terminal inteiro (height é só o corpo: o app já
  // gastou 4 linhas com cabeçalho e taskbar)
  const need = `${matrix.length ? matrix[0].length : 0} colunas x ${qrHeight(matrix) + CHROME + 4} linhas`;
  return h(Box, {flexDirection: 'column', height, overflow: 'hidden', paddingX: 1},
    h(Text, {wrap: 'truncate'},
      h(Text, {color: theme.primary, bold: true}, 'PAREAR_APARELHO'),
      // com várias contas, deixa claro qual está sendo pareada
      accountLabel ? h(Text, {color: theme.secondary}, ` — CONTA: ${accountLabel.toUpperCase()}`) : null),
    h(Text, {color: theme.textDim, wrap: 'truncate'},
      qr?.message || 'aguardando o QR code do núcleo…'),
    roomy ? h(Text, null, ' ') : null,
    fits
      ? h(Box, {justifyContent: 'center', flexShrink: 0}, h(Code, {matrix}))
      : h(Box, {flexDirection: 'column', flexGrow: 1},
        matrix.length
          ? h(React.Fragment, null,
            h(Text, {color: theme.error, wrap: 'truncate'},
              `a janela é pequena para desenhar o QR (precisa de ${need}, tem ${width} x ${height + 4})`),
            h(Text, {color: theme.textDim, wrap: 'truncate'},
              'abrimos a imagem do código no visualizador do sistema — ou diminua a fonte'),
            h(Text, {color: theme.textDim, wrap: 'truncate'},
              'do terminal (Ctrl+-) e aumente a janela para lê-lo aqui'),
          )
          : h(Text, {color: theme.textDim}, 'gerando o código…'),
      ),
    roomy ? h(Text, null, ' ') : null,
    roomy && qr?.png
      ? h(Text, {color: theme.textDim, wrap: 'truncate'}, `imagem do QR: ${qr.png}`)
      : null,
    h(Box, {flexGrow: 1}),
    h(Text, {color: theme.secondary, wrap: 'truncate'},
      '[O] abrir imagem · [N] novo QR · [C] cancelar · [ESC] esconder · [CTRL+R] reconectar' +
        (accountLabel ? ' · [ALT+1-9] outra conta' : '')),
  );
}
