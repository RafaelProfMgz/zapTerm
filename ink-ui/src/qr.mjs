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
  return matrix[0].length <= width && Math.ceil(matrix.length / 2) <= height;
}

function Code({matrix}) {
  const rows = qrRuns(matrix);
  return h(Box, {flexDirection: 'column'},
    ...rows.map((runs, i) => h(Text, {key: i},
      ...runs.map((r, j) => h(Text, {key: j, color: r.fg, backgroundColor: r.bg}, '▀'.repeat(r.len))),
    )),
  );
}

export default function QRScreen({qr, height, width}) {
  const matrix = qr?.matrix || [];
  const fits = qrFits(matrix, width - 4, height - 8);
  return h(Box, {flexDirection: 'column', height, overflow: 'hidden', paddingX: 1},
    h(Text, {color: theme.primary, bold: true}, 'PAREAR_APARELHO'),
    h(Text, {color: theme.textDim, wrap: 'truncate'},
      qr?.message || 'aguardando o QR code do núcleo…'),
    h(Text, null, ' '),
    fits
      ? h(Box, {justifyContent: 'center'}, h(Code, {matrix}))
      : h(Text, {color: theme.error},
        matrix.length
          ? 'terminal pequeno demais para desenhar o QR — abra a imagem abaixo'
          : 'gerando o código…'),
    h(Text, null, ' '),
    qr?.png
      ? h(Text, {color: theme.textDim, wrap: 'truncate'}, `imagem do QR: ${qr.png}`)
      : null,
    h(Box, {flexGrow: 1}),
    h(Text, {color: theme.secondary},
      '[N] novo QR · [C] cancelar · [ESC] esconder · [CTRL+R] reconectar'),
  );
}
