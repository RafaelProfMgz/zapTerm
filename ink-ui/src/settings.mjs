import React from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

// Ações de conexão da tela CONFIG. Cada uma é um comando do núcleo Go: os
// mesmos que o usuário pode digitar no prompt (/reconectar, /novoqr…).
export const ACTIONS = [
  {key: 'R', cmd: 'reconectar', label: 'RECONECTAR'},
  {key: 'N', cmd: 'novoqr', label: 'NOVO_QR'},
  {key: 'D', cmd: 'disconnect', label: 'DESCONECTAR'},
  {key: 'L', cmd: 'logout', label: 'SAIR_DA_CONTA'},
  {key: 'Z', cmd: 'reset', label: 'RESETAR_SESSAO'},
];

// Linha (1-based, no terminal inteiro) em que os botões são desenhados:
// cabeçalho da tela (2) + linha em branco (1) + borda e título do cartão (2),
// contando a partir do topo do corpo (linha 3). Precisa bater com o JSX abaixo.
export const ACTIONS_ROW = 8;
// Primeira coluna do texto: padding da tela (1) + borda (1) + padding (1).
const ACTIONS_COL = 4;

export function actionLabel(a) {
  return `[${a.key}] ${a.label}`;
}

// settingsActionAt mapeia um clique do mouse no botão correspondente. Cada
// botão ocupa o rótulo mais um espaço de cada lado (o realce de fundo), com
// dois espaços de separação entre eles.
export function settingsActionAt(x, y) {
  if (y !== ACTIONS_ROW) return null;
  let cur = ACTIONS_COL;
  for (const a of ACTIONS) {
    const w = actionLabel(a).length + 2;
    if (x >= cur && x < cur + w) return a;
    cur += w + 2;
  }
  return null;
}

// settingsActionForKey resolve a tecla de atalho (maiúscula ou minúscula).
export function settingsActionForKey(input) {
  const k = String(input || '').toUpperCase();
  return ACTIONS.find(a => a.key === k) || null;
}

// Card: cartão do design de settings — título em caps separado do corpo,
// contorno de 1px, sem sombra. flexShrink fica em 0 por padrão: sem isso o
// yoga encolhe os primeiros cartões quando a tela não cabe e as linhas se
// sobrepõem — só o cartão de ATALHOS (shrink) pode ser cortado.
function Card({title, children, grow, shrink}) {
  return h(Box, {
    flexDirection: 'column',
    flexGrow: grow ? 1 : 0,
    flexBasis: grow ? 0 : undefined,
    flexShrink: shrink ? 1 : 0,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: theme.outlineDim,
    paddingX: 1,
    marginRight: 1,
  },
    h(Text, {color: theme.primary, bold: true}, `[${title}]`),
    ...children,
  );
}

function Field({label, value, color}) {
  return h(Text, {wrap: 'truncate'},
    h(Text, {color: theme.textDim, dimColor: true}, `${label}: `),
    h(Text, {color: color || theme.text}, value),
  );
}

function Key({k, action}) {
  return h(Text, {wrap: 'truncate'},
    h(Text, {color: theme.secondary, bold: true}, `[${k}]`.padEnd(10)),
    h(Text, {color: theme.textDim}, action),
  );
}

// Swatch: amostras do tema, como os "THEME_CORES" do design.
function Swatch({colors}) {
  return h(Text, null,
    ...colors.map((c, i) => h(Text, {key: i, backgroundColor: c}, '  ')),
  );
}

// Button: botão clicável (ou por tecla) da faixa de ações.
function Button({action, danger}) {
  return h(Text, {
    color: danger ? theme.error : theme.onPrimary,
    backgroundColor: danger ? undefined : theme.primary,
    bold: true,
  }, ` ${actionLabel(action)} `);
}

export default function SettingsScreen({version, status, binPath, height}) {
  const state = status.connecting
    ? 'CONECTANDO…'
    : status.connected ? 'CONECTADO' : status.loggedIn ? 'DESCONECTADO' : 'SEM SESSÃO';
  const stateColor = status.connected ? theme.primary : status.connecting ? theme.secondary : theme.error;
  return h(Box, {flexDirection: 'column', height, overflow: 'hidden', paddingX: 1},
    h(Box, {justifyContent: 'space-between', flexShrink: 0},
      h(Text, {color: theme.primary, bold: true}, 'CONFIG_DO_SISTEMA'),
      h(Text, {color: theme.textDim, dimColor: true}, 'clique nos botões ou use as teclas'),
    ),
    h(Box, {flexShrink: 0},
      h(Text, {color: theme.textDim, dimColor: true},
        'Interface de configuração do ZapTerm. Edite o INI abaixo e reinicie.'),
    ),
    h(Box, {flexShrink: 0}, h(Text, null, ' ')),
    // faixa de ações — a linha dos botões precisa continuar em ACTIONS_ROW
    h(Card, {title: 'CONEXÃO', children: [
      h(Text, {key: 'btns'},
        ...ACTIONS.map((a, i) => h(Text, {key: a.key},
          i > 0 ? h(Text, null, '  ') : null,
          h(Button, {action: a, danger: a.cmd === 'reset' || a.cmd === 'logout'}),
        )),
      ),
      h(Text, {key: 'hint', color: theme.textDim, dimColor: true, wrap: 'truncate'},
        'NOVO_QR apaga a sessão e abre um QR novo · RESETAR_SESSAO limpa tudo do zero'),
    ]}),
    h(Box, {flexShrink: 0},
      h(Card, {title: 'ARQUIVOS_DO_SISTEMA', grow: true, children: [
        h(Field, {key: 'cfg', label: 'CONFIG ', value: '~/.config/whatscli/whatscli.config'}),
        h(Field, {key: 'db', label: 'SESSÃO ', value: '~/.config/whatscli/accounts/default/session.db'}),
      ]}),
      h(Card, {title: 'NÚCLEO_GO', grow: true, children: [
        h(Field, {key: 'bin', label: 'BINÁRIO', value: binPath || '?'}),
        h(Field, {key: 'ver', label: 'VERSÃO ', value: version || '—', color: theme.secondary}),
        h(Field, {key: 'st', label: 'ESTADO ', value: state, color: stateColor}),
      ]}),
    ),
    h(Box, {flexShrink: 0},
      h(Card, {title: 'NÚCLEOS_DE_TEMA', grow: true, children: [
        h(Text, {key: 'n', color: theme.text}, 'TERMINAL_PROTOCOL — âmbar sobre terra'),
        h(Box, {key: 'sw'},
          h(Swatch, {colors: [theme.primary, theme.primaryDim, theme.secondary, theme.tertiary, theme.text, theme.surfaceHighest]}),
          h(Text, {color: theme.primary, bold: true}, '  ● ATIVO'),
        ),
      ]}),
      h(Card, {title: 'FRONTEND', grow: true, children: [
        h(Field, {key: 'ui', label: 'UI     ', value: 'ink (experimental)'}),
        h(Field, {key: 'ponte', label: 'PONTE  ', value: 'NDJSON via stdio', color: theme.tertiary}),
      ]}),
    ),
    h(Box, {flexGrow: 1, flexShrink: 1, overflow: 'hidden'},
      h(Card, {title: 'ATALHOS', grow: true, shrink: true, children: [
        h(Key, {key: 'f', k: 'F1-F4', action: 'telas: sessão · túnel · logs · config'}),
        h(Key, {key: 'tab', k: 'TAB', action: 'alterna painéis (conversas → mensagens → digitação)'}),
        h(Key, {key: 'nav', k: '↑/↓', action: 'navegar / rolar logs'}),
        h(Key, {key: 'ent', k: 'ENTER', action: 'abrir conversa · enviar mensagem · tocar áudio'}),
        h(Key, {key: 'find', k: 'CTRL+F', action: 'buscar contato por nome ou número (TAB alterna escopo)'}),
        h(Key, {key: 'fil', k: '1-4 / F', action: 'filtros de conversa: todas · não lidas · grupos · contatos'}),
        h(Key, {key: 'pod', k: 'P/O/D', action: 'tocar áudio · abrir anexo · baixar (mensagem selecionada)'}),
        h(Key, {key: 'b', k: 'B', action: 'carregar histórico (backlog)'}),
        h(Key, {key: 'rec', k: 'CTRL+R', action: 'reconectar ao WhatsApp de qualquer tela'}),
        h(Key, {key: 'cmd', k: '/CMD', action: 'comando do núcleo Go (/reconectar, /novoqr, /openqr, /read…)'}),
        h(Key, {key: 'q', k: 'CTRL+Q', action: 'sair'}),
      ]}),
    ),
  );
}
