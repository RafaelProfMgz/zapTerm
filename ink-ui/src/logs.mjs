import React from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

// Cada linha de log tem uma tag estilo protocolo, como no design:
// [SISTEMA] âmbar, [REDE] ciano, [ERRO] vermelho-claro.
function tagOf(line) {
  switch (line.kind) {
    case 'error': return {label: '[ERRO]   ', color: theme.error};
    case 'net': return {label: '[REDE]   ', color: theme.tertiary};
    default: return {label: '[SISTEMA]', color: theme.primaryDim};
  }
}

function LogLine({line}) {
  const tag = tagOf(line);
  return h(Text, {wrap: 'truncate-end'},
    h(Text, {color: theme.outlineDim}, `[${line.stamp || '--:--:--'}] `),
    h(Text, {color: tag.color}, tag.label),
    h(Text, {color: line.kind === 'error' ? theme.error : theme.text}, ` ${line.text || ' '}`),
  );
}

// MetricCard: cartão do bento grid inferior — rótulo em caps, valor grande
// colorido e uma barra de "nível" minimalista.
function MetricCard({label, value, color, level}) {
  const filled = Math.max(0, Math.min(10, level));
  return h(Box, {
    flexDirection: 'column',
    flexGrow: 1,
    flexBasis: 0,
    paddingX: 1,
    borderStyle: 'single',
    borderColor: theme.outlineDim,
  },
    h(Text, {color: theme.textDim, dimColor: true}, label),
    h(Text, {color, bold: true, wrap: 'truncate'}, value),
    h(Text, {color}, '▰'.repeat(filled) + '▱'.repeat(10 - filled)),
  );
}

export default function LogsScreen({log, status, scroll, height}) {
  const errors = log.filter(l => l.kind === 'error').length;
  // altura: cabeçalho(2) + painel(borda 2 + título 1 + cursor 1) + métricas(4)
  const visible = Math.max(1, height - 10);
  const end = Math.max(0, log.length - scroll);
  const slice = log.slice(Math.max(0, end - visible), end);

  return h(Box, {flexDirection: 'column', height, overflow: 'hidden'},
    // cabeçalho da tela
    h(Box, {justifyContent: 'space-between', paddingX: 1},
      h(Text, {color: theme.primary, bold: true}, 'SAÍDA_DO_SISTEMA'),
      h(Text, null,
        h(Text, {color: theme.tertiary}, `● EVENTOS: ${log.length}  `),
        h(Text, {color: theme.primary, bold: true},
          scroll > 0 ? `↑ ROLADO ${scroll}` : '● FEED_AO_VIVO'),
      ),
    ),
    h(Box, {paddingX: 1},
      h(Text, {color: theme.textDim, dimColor: true},
        'FEED DE TELEMETRIA EM TEMPO REAL // NÚCLEO_GO — [↑/↓] rolar'),
    ),
    // console
    h(Box, {
      flexDirection: 'column',
      flexGrow: 1,
      overflow: 'hidden',
      borderStyle: 'single',
      borderColor: theme.outlineDim,
      paddingX: 1,
    },
      h(Box, {justifyContent: 'space-between'},
        h(Text, {color: theme.textDim, bold: true}, '[ CONSOLE ]'),
        h(Text, {color: theme.textDim, dimColor: true}, 'FILTRO: TODOS_EVENTOS'),
      ),
      ...slice.map((line, i) => h(LogLine, {key: `${end - slice.length + i}`, line})),
      scroll === 0
        ? h(Text, null,
            h(Text, {color: theme.primary}, '> '),
            h(Text, {color: theme.primaryDim}, '▮'))
        : null,
    ),
    // bento grid de métricas
    h(Box, null,
      h(MetricCard, {
        label: 'EVENTOS',
        value: String(log.length),
        color: theme.tertiary,
        level: Math.min(10, Math.ceil(log.length / 30)),
      }),
      h(MetricCard, {
        label: 'ERROS',
        value: String(errors),
        color: errors > 0 ? theme.error : theme.secondary,
        level: errors > 0 ? Math.min(10, errors) : 10,
      }),
      h(MetricCard, {
        label: 'INTEGRIDADE',
        value: status.connected ? 'ESTÁVEL' : 'SEM_SINAL',
        color: status.connected ? theme.primary : theme.error,
        level: status.connected ? 10 : 0,
      }),
    ),
  );
}
