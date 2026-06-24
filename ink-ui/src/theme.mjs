// Tema "Terminal Protocol" (design/terminal_protocol/DESIGN.md): âmbar
// queimado sobre terra escura — retro-técnico calmo, sem glow. Cor é usada
// com parcimônia: âmbar só em foco/prompt/destaque, ciano só em status de
// rede, o resto fica em off-white quente e contornos terrosos.
export default {
  // superfícies (o fundo real é o do terminal; estas servem p/ seleção)
  bg: '#19120c',
  surface: '#261e18',
  surfaceHigh: '#312822',
  surfaceHighest: '#3c332c',

  // texto: off-white quente; variante p/ metadados, timestamps e dicas
  text: '#efe0d6',
  textDim: '#d9c2b2',

  // contornos: claro p/ destaque estrutural, escuro p/ painéis em repouso
  outline: '#a18d7e',
  outlineDim: '#544437',

  // âmbar queimado: cursor, prompt, painel focado, caminho crítico
  primary: '#ffbb82',
  primaryDim: '#f29946',
  onPrimary: '#4c2700',

  // marrom-areia: acentos estruturais, horários, dados secundários
  secondary: '#efbc94',
  secondaryDim: '#dcab84',

  // ciano: exclusivo p/ status de sistema/rede ([ONLINE], túnel, telemetria)
  tertiary: '#55d9f5',

  error: '#ffb4ab',

  // nomes de contato: tons quentes estáveis + ciano, estilo IRC
  peers: ['#efbc94', '#f29946', '#ffdcc2', '#55d9f5', '#dcab84', '#a8edff'],
};

// contactColor devolve um tom estável por nome (estilo IRC).
export function contactColor(name, palette) {
  let hash = 0;
  for (const ch of String(name)) hash = (hash * 31 + ch.codePointAt(0)) >>> 0;
  return palette.peers[hash % palette.peers.length];
}
