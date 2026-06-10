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

  // verdes só onde importa: não lidas/conversa aberta; o resto fica calmo
  const nameColor = chat.unread > 0 ? theme.accent : chat.isGroup ? theme.sage : theme.text;
  return h(Box, {justifyContent: 'space-between', width: width - 2},
    h(Text, {
      color: isSelected ? theme.bg : nameColor,
      backgroundColor: isSelected ? theme.green : undefined,
      bold: chat.unread > 0 || isCurrent,
      wrap: 'truncate',
    }, name),
    h(Text, {color: chat.unread > 0 ? theme.accent : theme.clay, bold: chat.unread > 0}, right),
  );
}

export default function ChatList({chats, filter, selected, currentId, focused, height, width}) {
  const visible = Math.max(1, height - 3);
  // janela de rolagem em torno da seleção
  let start = Math.max(0, Math.min(selected - Math.floor(visible / 2), chats.length - visible));
  if (start < 0) start = 0;
  const slice = chats.slice(start, start + visible);

  return h(Box, {
    flexDirection: 'column',
    width,
    borderStyle: 'single',
    borderColor: focused ? theme.accent : theme.clayDark,
  },
    h(Text, {color: focused ? theme.accent : theme.clay, bold: true},
      ` [ ${FILTERS[filter].toUpperCase()} (${chats.length}) ]`),
    h(Text, {color: theme.clayDark}, ` 1-4 filtra · f alterna`),
    ...slice.map((c, i) => h(Row, {
      key: c.id,
      chat: c,
      isSelected: focused && start + i === selected,
      isCurrent: c.id === currentId,
      width,
    })),
  );
}
