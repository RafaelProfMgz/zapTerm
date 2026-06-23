// stories.mjs — tela dedicada aos STORIES (status@broadcast), separada da
// lista de conversas. À esquerda, os autores com status recentes; à direita,
// os posts do autor selecionado. O núcleo Go agrupa por remetente e manda
// tudo no evento "stories" (inclui as mensagens), então aqui é só render.
import React from 'react';
import {Box, Text} from 'ink';
import theme, {contactColor} from './theme.mjs';
import {shortTime} from './chatlist.mjs';

const h = React.createElement;

// kindLabel: rótulo curto pt-BR por tipo de mídia do status.
export function kindLabel(kind) {
  switch (kind) {
    case 'image': return '[imagem]';
    case 'video': return '[vídeo]';
    case 'audio': return '[áudio]';
    case 'document': return '[arquivo]';
    default: return '';
  }
}

// storyPreview: texto curto de um post de status para a lista/visualização.
export function storyPreview(msg) {
  const label = kindLabel(msg.kind);
  const text = (msg.text || '').trim();
  if (label && text && text !== label) return `${label} ${text}`;
  if (label) return label;
  return text || '(sem legenda)';
}

function stamp(ts) {
  return new Date(ts * 1000).toTimeString().slice(0, 8);
}

function truncate(s, max) {
  if (s.length <= max) return s;
  return s.slice(0, Math.max(0, max - 1)) + '…';
}

// SenderRow: uma linha da coluna esquerda — autor, nº de posts e horário do
// último; bolinha cheia (●) quando há status não vistos.
function SenderRow({story, isSelected, width}) {
  const dot = story.unread > 0 ? '● ' : '○ ';
  const name = story.name || story.short || `+${String(story.senderId).replace(/@.*/, '')}`;
  const right = `${story.messages.length}·${shortTime(story.lastMessage)}`;
  const avail = Math.max(4, width - right.length - 3);
  const nameColor = isSelected ? theme.onPrimary
    : story.unread > 0 ? theme.primary
    : contactColor(name, theme);
  return h(Box, {justifyContent: 'space-between', width: width - 2},
    h(Text, {
      color: nameColor,
      backgroundColor: isSelected ? theme.primary : undefined,
      bold: story.unread > 0 || isSelected,
      wrap: 'truncate',
    }, truncate(dot + name, avail)),
    h(Text, {
      color: isSelected ? theme.primary : theme.secondaryDim,
      dimColor: !isSelected,
    }, right),
  );
}

// PostLine: um post na coluna direita, estilo IRC "[hh:mm:ss] preview".
function PostLine({msg}) {
  return h(Box, {paddingLeft: 1},
    h(Text, {color: theme.textDim, dimColor: true}, `[${stamp(msg.timestamp)}] `),
    h(Text, {color: theme.text, wrap: 'truncate-end'}, storyPreview(msg)),
  );
}

export default function StoriesScreen({stories, selected, focused, height, width}) {
  const cols = Math.max(28, width != null ? Math.floor(width * 0.4) : 34);
  const sel = Math.max(0, Math.min(selected || 0, stories.length - 1));
  const current = stories[sel];
  const listVisible = Math.max(1, height - 4);
  const start = Math.max(0, Math.min(sel - Math.floor(listVisible / 2), Math.max(0, stories.length - listVisible)));
  const slice = stories.slice(start, start + listVisible);

  // coluna esquerda: autores com status
  const left = h(Box, {
    flexDirection: 'column',
    width: cols,
    flexShrink: 0,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: focused ? theme.primary : theme.outlineDim,
  },
    h(Text, {color: focused ? theme.primary : theme.textDim, bold: true},
      ' [ STORIES ]'),
    h(Text, {color: theme.textDim, dimColor: true},
      ` ATUALIZAÇÕES: ${stories.length}`),
    ...slice.map((s, i) => h(SenderRow, {
      key: s.senderId,
      story: s,
      isSelected: start + i === sel,
      width: cols,
    })),
    stories.length === 0
      ? h(Text, {color: theme.outlineDim, dimColor: true}, ' SEM_STORIES')
      : null,
  );

  // coluna direita: posts do autor selecionado
  const postVisible = Math.max(1, height - 3);
  const posts = current ? current.messages.slice(-postVisible) : [];
  const right = h(Box, {
    flexDirection: 'column',
    flexGrow: 1,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: theme.outlineDim,
  },
    h(Text, {wrap: 'truncate'},
      h(Text, {color: theme.primary, bold: true},
        ` [ ${(current ? (current.name || current.short || 'STATUS') : 'STATUS').toUpperCase()} ]`),
      current
        ? h(Text, {color: theme.textDim, dimColor: true}, ` // ${current.messages.length} post(s)`)
        : null,
    ),
    ...(current
      ? posts.map((m, i) => h(PostLine, {key: m.id || `${m.timestamp}-${i}`, msg: m}))
      : [h(Text, {key: 'empty', color: theme.outlineDim, dimColor: true},
          ' selecione um contato à esquerda para ver os status')]),
  );

  return h(Box, {flexDirection: 'column', height, overflow: 'hidden'},
    h(Box, {flexGrow: 1}, left, right),
    h(Box, {paddingX: 1},
      h(Text, {color: theme.textDim, dimColor: true, wrap: 'truncate'},
        '[↑/↓] navegar contatos · [F1] voltar à sessão · stories expiram em 24h no WhatsApp'),
    ),
  );
}
