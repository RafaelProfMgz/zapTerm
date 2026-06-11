import React from 'react';
import {Box, Text} from 'ink';
import theme, {contactColor} from './theme.mjs';

const h = React.createElement;

const WEEKDAYS = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];

function stamp(ts) {
  return new Date(ts * 1000).toTimeString().slice(0, 8);
}

function dayOf(ts) {
  return new Date(ts * 1000).toDateString();
}

function daySeparator(ts) {
  const t = new Date(ts * 1000);
  const d = `${String(t.getDate()).padStart(2, '0')}/${String(t.getMonth() + 1).padStart(2, '0')}/${t.getFullYear()}`;
  return `<<< ${WEEKDAYS[t.getDay()]}, ${d} >>>`;
}

function fmtDuration(secs) {
  return `${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, '0')}`;
}

function mediaHint(msg) {
  switch (msg.kind) {
    case 'audio': {
      let hint = '[p/Enter] tocar · [o] abrir';
      if (msg.durationSecs > 0) hint = `${fmtDuration(msg.durationSecs)} · ${hint}`;
      return hint;
    }
    case 'video': {
      let hint = '[p] tocar · [o] abrir';
      if (msg.durationSecs > 0) hint = `${fmtDuration(msg.durationSecs)} · ${hint}`;
      return hint;
    }
    case 'image': return '[o] abrir';
    case 'document': return '[o] abrir · [d] baixar';
    default: return '';
  }
}

// MessageLine: estilo IRC "[15:04:05] <nome> texto" em colunas, como o
// design — timestamp e nome são colunas fixas, o texto quebra alinhado.
function MessageLine({msg, isSelected, isPlaying}) {
  const who = msg.fromMe ? '<eu>' : `<${msg.contactShort || msg.contactName || '?'}>`;
  const whoColor = msg.fromMe ? theme.primary : contactColor(msg.contactShort || msg.contactName, theme);
  const hint = mediaHint(msg);
  const bg = isSelected ? theme.surfaceHighest : undefined;
  return h(Box, {paddingLeft: 1},
    h(Box, {flexShrink: 0},
      h(Text, {color: theme.textDim, dimColor: true, backgroundColor: bg},
        `[${stamp(msg.timestamp)}] `),
      h(Text, {color: whoColor, bold: true, backgroundColor: bg}, who),
    ),
    h(Box, {flexGrow: 1, flexBasis: 0},
      h(Text, {wrap: 'wrap', backgroundColor: bg},
        h(Text, {
          color: msg.forwarded ? theme.secondaryDim : theme.text,
          italic: msg.forwarded,
        }, ` ${msg.text}`),
        hint ? h(Text, {color: theme.textDim, dimColor: true}, `  (${hint})`) : null,
        isPlaying ? h(Text, {color: theme.primary, bold: true}, '  ▶ TOCANDO') : null,
      ),
    ),
  );
}

// LogView: tela de boot/login — mostra as linhas de log do Go (inclui o QR
// code de login, por isso precisa de área inteira, não rodapé).
export function LogView({log, height}) {
  const slice = log.slice(-(Math.max(1, height - 2)));
  return h(Box, {flexDirection: 'column'},
    ...slice.map((line, i) => h(Text, {
      key: i,
      color: line.kind === 'error' ? theme.error : theme.textDim,
      wrap: 'truncate-end',
    }, ` ${line.text || ''}`)),
  );
}

export default function Messages({msgs, log, chatName, selected, playingId, focused, height}) {
  const body = [];
  if (msgs.length === 0) {
    body.push(h(LogView, {key: 'log', log, height}));
  } else {
    // janela: mostra as últimas linhas que cabem (ou em volta da seleção)
    const visible = Math.max(1, height - 3);
    let start = Math.max(0, msgs.length - visible);
    if (selected != null && selected < start) start = selected;
    const slice = msgs.slice(start, start + visible);
    let lastDay = start > 0 ? dayOf(msgs[start - 1].timestamp) : '';
    slice.forEach((m, i) => {
      const day = dayOf(m.timestamp);
      if (day !== lastDay) {
        body.push(h(Text, {
          key: `sep-${m.id}`,
          color: theme.outlineDim,
        }, ` ${daySeparator(m.timestamp)}`));
        lastDay = day;
      }
      body.push(h(MessageLine, {
        key: m.id || `${m.timestamp}-${i}`,
        msg: m,
        isSelected: focused && selected === start + i,
        isPlaying: playingId && m.id === playingId,
      }));
    });
  }

  return h(Box, {
    flexDirection: 'column',
    flexGrow: 1,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: focused ? theme.primary : theme.outlineDim,
  },
    h(Text, {wrap: 'truncate'},
      h(Text, {color: focused ? theme.primary : theme.textDim, bold: true},
        ` [ ${(chatName || 'MENSAGENS').toUpperCase()} ]`),
      chatName
        ? h(Text, {color: theme.textDim, dimColor: true}, ' // sessão_criptografada')
        : null,
    ),
    ...body,
  );
}
