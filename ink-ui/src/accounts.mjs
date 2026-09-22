// accounts.mjs — várias contas no Ink: o estado de cada conta (byAccount), o
// hook que funde os eventos do núcleo nele e o trilho lateral [ CONTAS ].
import React, {useEffect, useRef, useState} from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

export const RAIL_WIDTH = 20;

// o trilho sempre aparece com mais de uma conta; com uma só, só quando sobra
// largura (ele serve de atalho para "+ nova conta")
export function railVisible(accounts, cols) {
  return accounts.length > 1 || cols >= 110;
}

export const EMPTY_ACCOUNT = Object.freeze({
  chats: [],
  stories: [],
  msgs: [],
  status: {connected: false, lastSeen: '', loggedIn: false, connecting: false, needsLogin: false},
  qr: null,
  currentChat: null,
});

export function unreadOf(chats) {
  return (chats || []).reduce((n, c) => n + (c.unread > 0 ? c.unread : 0), 0);
}

// statusTag: mesmas tags do rodapé ([ONLINE]/[CONECTANDO]/[SEM SESSÃO]/[OFFLINE])
export function statusTag(st) {
  if (!st) return '[OFFLINE]';
  if (st.connected) return '[ONLINE]';
  if (st.connecting) return '[CONECTANDO]';
  if (st.needsLogin || !st.loggedIn) return '[SEM SESSÃO]';
  return '[OFFLINE]';
}

// railItemAt mapeia a linha clicada (relativa ao topo do painel, borda = 0)
// para uma conta ou para "+ nova conta" — precisa concordar com AccountRail:
// borda, título, e 2 linhas por conta.
export function railItemAt(relY, count) {
  const i = relY - 2;
  if (i < 0) return null;
  if (i < count * 2) return {kind: 'account', index: Math.floor(i / 2)};
  if (i === count * 2) return {kind: 'add'};
  return null;
}

// nextAccount: Ctrl+↑/↓ anda na lista, dando a volta nas pontas
export function nextAccount(accounts, activeId, delta) {
  if (!accounts.length) return null;
  const i = Math.max(0, accounts.findIndex(a => a.id === activeId));
  return accounts[(i + delta + accounts.length) % accounts.length].id;
}

function truncate(s, max) {
  return s.length <= max ? s : s.slice(0, Math.max(0, max - 1)) + '…';
}

export function AccountRail({accounts, activeId, byAccount, height}) {
  const inner = RAIL_WIDTH - 2;
  return h(Box, {
    flexDirection: 'column',
    width: RAIL_WIDTH,
    height,
    flexShrink: 0,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: theme.outlineDim,
  },
    h(Text, {color: theme.textDim, bold: true}, '[ CONTAS ]'),
    ...accounts.map((acc, i) => {
      const isActive = acc.id === activeId;
      const unread = unreadOf(byAccount[acc.id]?.chats);
      // estado vindo do evento "accounts" (o núcleo republica a cada mudança)
      const st = acc;
      const tag = statusTag(st);
      return h(Box, {key: acc.id, flexDirection: 'column'},
        h(Text, {
          color: isActive ? theme.primary : theme.text,
          bold: isActive,
          wrap: 'truncate',
        }, truncate(`${isActive ? '*' : ' '} ${i + 1} ${acc.label || acc.id}`, inner)),
        h(Text, {wrap: 'truncate'},
          '  ',
          unread > 0 ? h(Text, {color: theme.primary, bold: true}, `[${unread}] `) : null,
          h(Text, {
            color: st.connected ? theme.tertiary : st.connecting ? theme.secondary : theme.error,
            dimColor: !isActive,
          }, tag),
        ),
      );
    }),
    accounts.length < 5
      ? h(Text, {color: theme.secondary, dimColor: true}, '+ nova conta')
      : null,
  );
}

// useAccounts guarda o estado de cada conta (byAccount[id]) e funde nele os
// eventos do núcleo: todo evento de sessão traz "account" (sem ele — núcleo
// antigo — vale a conta ativa). Assim as outras contas seguem recebendo
// conversas e mensagens enquanto a ativa está aberta (ou mostrando o QR).
// onLog(kind, text, accountId) vai para o feed; onSwitch(id, state) roda na
// troca de conta ativa.
export function useAccounts(bridge, {onLog, onSwitch}) {
  const [accounts, setAccounts] = useState([]);
  const [activeId, setActiveId] = useState('');
  const [byAccount, setByAccount] = useState({});
  const activeRef = useRef('');
  const byRef = useRef({});
  const connRef = useRef({}); // último "connected" por conta, p/ logar transições
  const cb = useRef({onLog, onSwitch});
  cb.current = {onLog, onSwitch};

  useEffect(() => {
    const idOf = e => e.account || activeRef.current;
    const patch = (id, fn) => setByAccount(all => {
      const cur = all[id] || EMPTY_ACCOUNT;
      const next = fn(cur);
      if (!next) return all;
      const out = {...all, [id]: {...cur, ...next}};
      byRef.current = out;
      return out;
    });

    bridge.on('accounts', e => setAccounts(e.accounts || []));
    bridge.on('account', e => {
      const id = e.id || '';
      activeRef.current = id;
      setActiveId(id);
      const state = byRef.current[id] || EMPTY_ACCOUNT;
      // o núcleo limpa o chat aberto ao trocar de conta: pedimos de volta a
      // conversa que estava aberta nesta conta
      if (state.currentChat) bridge.send('select', [state.currentChat.id], id);
      cb.current.onSwitch?.(id, state);
    });
    bridge.on('chats', e => patch(idOf(e), () => ({chats: e.chats || []})));
    bridge.on('stories', e => patch(idOf(e), () => ({stories: e.stories || []})));
    bridge.on('screen', e => patch(idOf(e), () => ({msgs: e.messages || []})));
    bridge.on('message', e => patch(idOf(e), s =>
      e.message && s.currentChat && e.message.chatId === s.currentChat.id
        ? {msgs: [...s.msgs, e.message]}
        : null));
    bridge.on('status', e => {
      const id = idOf(e);
      const connected = !!e.connected;
      if (connRef.current[id] !== connected) {
        const first = connRef.current[id] === undefined;
        connRef.current[id] = connected;
        if (!(first && !connected)) {
          cb.current.onLog?.('net', connected
            ? '[TÚNEL_ESTABELECIDO] conexão estável.'
            : '[TÚNEL_PERDIDO] aguardando reconexão…', id);
        }
      }
      patch(id, () => ({
        status: {
          connected,
          lastSeen: e.lastSeen || '',
          loggedIn: !!e.loggedIn,
          connecting: !!e.connecting,
          needsLogin: !!e.needsLogin,
        },
      }));
    });
    bridge.on('qr', e => {
      const id = idOf(e);
      patch(id, () => ({
        qr: e.event === 'code' ? {matrix: e.matrix || [], png: e.png || '', message: e.message || ''} : null,
      }));
      if (e.message) cb.current.onLog?.('net', e.message, id);
    });
  }, [bridge]);

  // setters da conta ativa, no mesmo formato do useState (valor ou função)
  const setActiveField = field => value => {
    const id = activeRef.current;
    setByAccount(all => {
      const cur = all[id] || EMPTY_ACCOUNT;
      const v = typeof value === 'function' ? value(cur[field]) : value;
      const out = {...all, [id]: {...cur, [field]: v}};
      byRef.current = out;
      return out;
    });
  };

  return {
    accounts,
    activeId,
    byAccount,
    active: byAccount[activeId] || EMPTY_ACCOUNT,
    activeRef,
    setMsgs: setActiveField('msgs'),
    setCurrentChat: setActiveField('currentChat'),
    setQr: setActiveField('qr'),
  };
}
