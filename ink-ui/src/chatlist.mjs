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

export function filterChats(chats, filter) {
  const hasActivity = chats.some(c => c.lastMessage > 0);
  switch (filter) {
    case 1: return chats.filter(c => c.unread > 0);
    case 2: return chats.filter(c => c.isGroup);
    case 3: return chats.filter(c => !c.isGroup);
    default: return hasActivity ? chats.filter(c => c.lastMessage > 0) : chats;
  }
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

export default function ChatList({chats, filter, selected, currentId, focused, height, width}) {
  const visible = Math.max(1, height - 5);
  // janela de rolagem em torno da seleção
  let start = Math.max(0, Math.min(selected - Math.floor(visible / 2), chats.length - visible));
  if (start < 0) start = 0;
  const slice = chats.slice(start, start + visible);

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
    ...slice.map((c, i) => h(Row, {
      key: c.id,
      chat: c,
      isSelected: focused && start + i === selected,
      isCurrent: c.id === currentId,
      width,
    })),
    chats.length === 0
      ? h(Text, {color: theme.outlineDim, dimColor: true}, ' FIM_DA_LISTA')
      : null,
  );
}
