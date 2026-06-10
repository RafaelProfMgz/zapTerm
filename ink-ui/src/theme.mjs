// Paleta do ZapTerm Ink UI — mistura de verdes com cinza clay, para fugir do
// "tudo verde": o verde brilhante é só destaque; texto corrido e cromo
// (bordas, horários, dicas) ficam em tons de cinza clay.
export default {
  bg: '#141413',

  // texto corrido — cinza claro quente, NÃO verde
  text: '#d8d4cb',
  // cinza clay: timestamps, dicas, metadados
  clay: '#a39a8c',
  // clay escuro: bordas de painel sem foco, separadores
  clayDark: '#6b655a',

  // verde brilhante: só para destaques (painel focado, não lidas, tocando, eu)
  accent: '#00e639',
  // verde médio para elementos secundários ativos
  green: '#2ca62a',
  // verde-sage (acinzentado) para grupos e rótulos calmos
  sage: '#84967e',
  // minhas mensagens: verde claro suave
  me: '#9ef58a',

  // nomes de contato: cada um ganha um tom estável da paleta de verdes
  greens: ['#69df5c', '#3fbf6b', '#9bd77f', '#4cd98a', '#7fae5f', '#2ca62a'],

  danger: '#ff6f61',
};

// contactColor devolve um tom de verde estável por nome (estilo IRC).
export function contactColor(name, palette) {
  let hash = 0;
  for (const ch of String(name)) hash = (hash * 31 + ch.codePointAt(0)) >>> 0;
  return palette.greens[hash % palette.greens.length];
}
