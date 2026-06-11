import React from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

export const FILTERS = ['Todas', 'Não lidas', 'Grupos', 'Contatos'];

const WEEKDAYS = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];

// shortTime: hoje → "15:04", última semana → dia, antigo → "02/01" (como o TUI).
export function shortTime(ts) {
  const t = new Date(ts * 1000);
  const now = new Date();
  if (t.toDateString() === now.toDateString()) {
    return t.toTimeString().slice(0, 5);
  }
  if (now - t < 7 * 24 * 3600 * 1000) return WEEKDAYS[t.getDay()];
  return `${String(t.getDate()).padStart(2, '0')}/${String(t.getMonth() + 1).padStart(2, '0')}`;
}

// sortChats: últimas conversadas primeiro; sem histórico vai pro fim, em
// ordem alfabética. O núcleo Go já manda assim, mas a UI garante a ordem.
export function sortChats(chats) {
  return [...chats].sort((a, b) =>
    (b.lastMessage || 0) - (a.lastMessage || 0) ||
    String(a.name).localeCompare(String(b.name)));
}

export function filterChats(chats, filter) {
  const sorted = sortChats(chats);
  switch (filter) {
    case 1: return sorted.filter(c => c.unread > 0);
    case 2: return sorted.filter(c => c.isGroup);
    case 3: return sorted.filter(c => !c.isGroup);
    default: return sorted;
  }
}

// chatRows monta as linhas visíveis da lista: na fronteira entre as conversas
// com histórico e os contatos sem conversa entra uma linha separadora.
// Cada linha é {chat, index} (index aponta para o array de chats) ou {sep}.
export function chatRows(chats) {
  const rows = [];
  chats.forEach((chat, index) => {
    if (index > 0 && chats[index - 1].lastMessage > 0 && !(chat.lastMessage > 0)) {
      rows.push({sep: true});
    }
    rows.push({chat, index});
  });
  return rows;
}

// rowScrollStart: janela de rolagem centrada na linha selecionada — usada
// pelo render e pelo mapeamento de cliques do mouse (precisam concordar).
export function rowScrollStart(rowCount, selRow, visible) {
  return Math.max(0, Math.min(selRow - Math.floor(visible / 2), rowCount - visible));
}

function truncate(s, max) {
  if (s.length <= max) return s;
  return s.slice(0, Math.max(0, max - 1)) + '…';
}

function Row({chat, isSelected, isCurrent, width}) {
  const right = chat.unread > 0 ? `[${chat.unread}]` : chat.lastMessage > 0 ? shortTime(chat.lastMessage) : '';
  const marker = isCurrent ? '* ' : '  ';
  let name = chat.name || `+${chat.id.replace(/@.*/, '')}`;
  if (chat.isGroup) name = `# ${name}`;
  const avail = Math.max(4, width - right.length - 3);
  name = truncate(marker + name, avail);

  // âmbar só onde importa: não lidas e conversa aberta; o resto fica calmo
  const nameColor = chat.unread > 0 ? theme.primary
    : isCurrent ? theme.primary
    : chat.isGroup ? theme.secondaryDim
    : theme.text;
  return h(Box, {justifyContent: 'space-between', width: width - 2},
    h(Text, {
      color: isSelected ? theme.onPrimary : nameColor,
      backgroundColor: isSelected ? theme.primary : undefined,
      bold: chat.unread > 0 || isCurrent,
      wrap: 'truncate',
    }, name),
    h(Text, {
      color: chat.unread > 0 ? theme.primary : theme.secondaryDim,
      bold: chat.unread > 0,
      dimColor: chat.unread === 0,
    }, right),
  );
}

// FilterTabs: linha de abas estilo READY/IDLE/BUSY do design — a ativa fica
// invertida (âmbar sobre escuro), as demais apagadas.
function FilterTabs({filter}) {
  const parts = [];
  FILTERS.forEach((f, i) => {
    if (i > 0) parts.push(h(Text, {key: `sep-${i}`, color: theme.outlineDim}, '│'));
    parts.push(h(Text, {
      key: f,
      color: i === filter ? theme.onPrimary : theme.textDim,
      backgroundColor: i === filter ? theme.primary : undefined,
      bold: i === filter,
      dimColor: i !== filter,
    }, f.toUpperCase().replace(' ', '_')));
  });
  return h(Text, null, ' ', ...parts);
}

function sepLine(width) {
  const label = ' ── SEM_HISTÓRICO ';
  return (label + '─'.repeat(Math.max(0, width - 2 - label.length))).slice(0, width - 2);
}

export default function ChatList({chats, filter, selected, currentId, focused, height, width}) {
  const visible = Math.max(1, height - 5);
  const rows = chatRows(chats);
  const selRow = Math.max(0, rows.findIndex(r => r.index === selected));
  const start = rowScrollStart(rows.length, selRow, visible);
  const slice = rows.slice(start, start + visible);

  return h(Box, {
    flexDirection: 'column',
    width,
    flexShrink: 0,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: focused ? theme.primary : theme.outlineDim,
  },
    h(Text, {color: focused ? theme.primary : theme.textDim, bold: true},
      ' [CONVERSAS_ATIVAS]'),
    h(Text, {color: theme.textDim, dimColor: true},
      ` PEERS_CONECTADOS: ${chats.length}`),
    h(FilterTabs, {filter}),
    ...slice.map(r => r.sep
      ? h(Text, {key: 'sep', color: theme.outlineDim, dimColor: true}, sepLine(width))
      : h(Row, {
        key: r.chat.id,
        chat: r.chat,
        isSelected: focused && r.index === selected,
        isCurrent: r.chat.id === currentId,
        width,
      })),
    chats.length === 0
      ? h(Text, {color: theme.outlineDim, dimColor: true}, ' FIM_DA_LISTA')
      : null,
  );
}
