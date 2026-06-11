// finder.mjs — buscador de contatos estilo Telescope (Ctrl+F), espelhando o
// finder.go da UI padrão: fuzzy match por nome ou número, escopos
// Todos/Contatos/Grupos (Tab), Enter abre a conversa.
import React from 'react';
import {Box, Text} from 'ink';
import {TextInput} from '@inkjs/ui';
import theme from './theme.mjs';
import {rowScrollStart} from './chatlist.mjs';

const h = React.createElement;

export const FINDER_SCOPES = ['TODOS', 'CONTATOS', 'GRUPOS'];

// rawJid tira o sufixo do WhatsApp, sobrando o número ou id de grupo puro.
export function rawJid(id) {
  return String(id).replace(/@.*$/, '');
}

// fuzzyMatch: mesma lógica do finder.go — cada caractere da busca precisa
// aparecer no alvo em ordem (subsequência, case-insensitive); sequências
// contíguas e matches no começo pontuam mais. Busca vazia casa com tudo.
export function fuzzyMatch(query, target) {
  if (!String(query).trim()) return {score: 0, ok: true};
  const q = [...String(query).toLowerCase()];
  const t = [...String(target).toLowerCase()];
  let qi = 0, score = 0, prev = -2;
  for (let ti = 0; ti < t.length && qi < q.length; ti++) {
    if (t[ti] === q[qi]) {
      if (ti === prev + 1) score += 5; // match contíguo
      if (ti < 10) score += 10 - ti; // match cedo = melhor
      prev = ti;
      qi++;
    }
  }
  if (qi < q.length) return {score: 0, ok: false};
  return {score, ok: true};
}

// searchChats filtra por escopo e fuzzy match sobre "nome + número/id",
// ordenando por relevância (e nome como desempate).
export function searchChats(chats, query, scope) {
  const matches = [];
  for (const chat of chats) {
    if (scope === 1 && chat.isGroup) continue;
    if (scope === 2 && !chat.isGroup) continue;
    const {score, ok} = fuzzyMatch(query, `${chat.name || ''} ${rawJid(chat.id)}`);
    if (ok) matches.push({chat, score});
  }
  matches.sort((a, b) => b.score - a.score ||
    String(a.chat.name).toLowerCase().localeCompare(String(b.chat.name).toLowerCase()));
  return matches.map(m => m.chat);
}

function truncate(s, max) {
  if (s.length <= max) return s;
  return s.slice(0, Math.max(0, max - 1)) + '…';
}

// ScopeTabs: abas TODOS│CONTATOS│GRUPOS, a ativa invertida em âmbar (mesmo
// padrão do FilterTabs da lista de conversas).
function ScopeTabs({scope}) {
  const parts = [];
  FINDER_SCOPES.forEach((s, i) => {
    if (i > 0) parts.push(h(Text, {key: `sep-${i}`, color: theme.outlineDim}, '│'));
    parts.push(h(Text, {
      key: s,
      color: i === scope ? theme.onPrimary : theme.textDim,
      backgroundColor: i === scope ? theme.primary : undefined,
      bold: i === scope,
      dimColor: i !== scope,
    }, s));
  });
  return h(Text, null, ...parts);
}

function ResultRow({chat, isSelected, width}) {
  const raw = rawJid(chat.id);
  let name = chat.name || `+${raw}`;
  if (chat.isGroup) name = `# ${name}`;
  if (chat.unread > 0) name += ` [${chat.unread}]`;
  const avail = Math.max(4, width - raw.length - 2);
  return h(Box, {justifyContent: 'space-between', width},
    h(Text, {
      color: isSelected ? theme.onPrimary : chat.isGroup ? theme.secondaryDim : theme.text,
      backgroundColor: isSelected ? theme.primary : undefined,
      bold: isSelected,
      wrap: 'truncate',
    }, truncate((isSelected ? '> ' : '  ') + name, avail)),
    h(Text, {color: isSelected ? theme.primary : theme.secondaryDim, dimColor: !isSelected}, raw),
  );
}

// Finder: painel centrado sobre a área da sessão. O estado (query, escopo,
// seleção) vive no App; aqui só desenha e devolve onChange/onSubmit do campo.
export default function Finder({results, scope, selected, height, width, inputKey, onChange, onSubmit}) {
  const inner = width - 4; // bordas (2) + paddingX (2)
  const listH = Math.max(3, height - 7);
  const selRow = Math.max(0, Math.min(selected, results.length - 1));
  const start = rowScrollStart(results.length, selRow, listH);
  const slice = results.slice(start, start + listH);

  return h(Box, {height, flexGrow: 1, alignItems: 'center', justifyContent: 'center'},
    h(Box, {
      flexDirection: 'column',
      width,
      borderStyle: 'single',
      borderColor: theme.primary,
      paddingX: 1,
    },
      h(Box, {justifyContent: 'space-between', width: inner},
        h(Text, {color: theme.primary, bold: true}, '[BUSCAR_CONTATO]'),
        h(ScopeTabs, {scope}),
      ),
      h(Box, null,
        h(Text, {color: theme.primary, bold: true}, 'busca> '),
        h(TextInput, {
          key: inputKey,
          placeholder: 'nome, número ou id…',
          onChange,
          onSubmit,
        }),
      ),
      h(Text, {color: theme.outlineDim, dimColor: true}, '─'.repeat(Math.max(0, inner))),
      ...slice.map((chat, i) => h(ResultRow, {
        key: chat.id,
        chat,
        isSelected: start + i === selRow,
        width: inner,
      })),
      results.length === 0
        ? h(Text, {color: theme.outlineDim, dimColor: true}, ' NENHUM_RESULTADO')
        : null,
      h(Text, {color: theme.textDim, dimColor: true, wrap: 'truncate'},
        `${results.length} resultado(s) · [↑/↓] navegar · [ENTER] abre · [TAB] escopo · [ESC] fecha`),
    ),
  );
}
