import React from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

// NodeBox: um nó do mapa do túnel (host local ⇄ rede WhatsApp).
function NodeBox({title, lines, color}) {
  return h(Box, {
    flexDirection: 'column',
    width: 26,
    paddingX: 1,
    borderStyle: 'single',
    borderColor: color,
  },
    h(Text, {color, bold: true}, title),
    ...lines.map((l, i) => h(Text, {key: i, color: theme.textDim, wrap: 'truncate'}, l)),
  );
}

function StatusRow({label, value, color}) {
  return h(Text, {wrap: 'truncate'},
    h(Text, {color: theme.textDim, dimColor: true}, ` ${label.padEnd(20, '.')}: `),
    h(Text, {color: color || theme.text, bold: !!color}, value),
  );
}

export default function TunnelScreen({status, chats, version, log, height}) {
  const connected = status.connected;
  const unread = chats.reduce((n, c) => n + (c.unread || 0), 0);
  const linkColor = connected ? theme.primary : theme.error;
  // feed: o que sobrar da altura depois do mapa(5) + estado(8) + títulos(3)
  const feedLines = Math.max(1, height - 16);
  const feed = log.slice(-feedLines);

  return h(Box, {flexDirection: 'column', height, overflow: 'hidden'},
    h(Box, {justifyContent: 'space-between', paddingX: 1},
      h(Text, {color: theme.primary, bold: true}, 'MAPA_DO_TÚNEL'),
      h(Text, {color: connected ? theme.tertiary : theme.error, bold: true},
        connected ? '[TÚNEL_ESTABELECIDO]' : '[TÚNEL_INATIVO]'),
    ),
    h(Box, {paddingX: 1},
      h(Text, {color: theme.textDim, dimColor: true},
        'VISUALIZAÇÃO DO ROTEAMENTO PONTA-A-PONTA // SESSÃO_CRIPTOGRAFADA'),
    ),
    // mapa: host local ⇄ rede WhatsApp
    h(Box, {paddingX: 1},
      h(NodeBox, {
        title: '[HOST_LOCAL]',
        lines: ['você@zapterm', `núcleo go ${version || '?'}`],
        color: theme.outline,
      }),
      h(Box, {flexDirection: 'column', justifyContent: 'center', paddingX: 2},
        h(Text, {color: theme.textDim, dimColor: true}, '  TÚNEL_E2E'),
        h(Text, {color: linkColor, bold: true},
          connected ? '═══════════▶' : '═══╳ ╳ ╳═══'),
      ),
      h(NodeBox, {
        title: '[REDE_WHATSAPP]',
        lines: ['servidor remoto', connected ? 'handshake ok' : 'aguardando…'],
        color: connected ? theme.outline : theme.outlineDim,
      }),
    ),
    // painel de estado
    h(Box, {
      flexDirection: 'column',
      borderStyle: 'single',
      borderColor: theme.outlineDim,
      paddingX: 1,
    },
      h(Text, {color: theme.textDim, bold: true}, '[ ESTADO_DO_TÚNEL ]'),
      h(StatusRow, {
        label: 'ESTADO',
        value: connected ? 'ESTABELECIDO' : 'DESCONECTADO',
        color: connected ? theme.primary : theme.error,
      }),
      h(StatusRow, {label: 'VISTO_POR_ÚLTIMO', value: status.lastSeen || '—'}),
      h(StatusRow, {label: 'PEERS_CONECTADOS', value: String(chats.length), color: theme.tertiary}),
      h(StatusRow, {label: 'NÃO_LIDAS', value: String(unread), color: unread > 0 ? theme.primary : undefined}),
      h(StatusRow, {label: 'VERSÃO_DO_NÚCLEO', value: version || '—', color: theme.secondary}),
    ),
    // feed de handshake (cauda do log)
    h(Box, {
      flexDirection: 'column',
      flexGrow: 1,
      overflow: 'hidden',
      borderStyle: 'single',
      borderColor: theme.outlineDim,
      paddingX: 1,
    },
      h(Text, {color: theme.textDim, bold: true}, '[ FEED_DE_HANDSHAKE ]'),
      ...feed.map((line, i) => h(Text, {
        key: i,
        color: line.kind === 'error' ? theme.error : theme.textDim,
        wrap: 'truncate-end',
      },
        h(Text, {color: theme.outlineDim}, `[${line.stamp || '--:--:--'}] `),
        line.text || ' ',
      )),
    ),
  );
}
