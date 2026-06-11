import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Box, Text, useApp, useInput, useStdin, useStdout} from 'ink';
import {TextInput} from '@inkjs/ui';
import {createBridge} from './bridge.mjs';
import theme from './theme.mjs';
import ChatList, {FILTERS, filterChats, chatRows, rowScrollStart} from './chatlist.mjs';
import Finder, {FINDER_SCOPES, searchChats} from './finder.mjs';
import Messages from './messages.mjs';
import TunnelScreen from './tunnel.mjs';
import LogsScreen from './logs.mjs';
import SettingsScreen from './settings.mjs';

const h = React.createElement;

const SIDEBAR_WIDTH = 34;
const FOCUS_ORDER = ['chats', 'messages', 'input'];

const SCREENS = [
  {id: 'session', label: 'SESSÃO'},
  {id: 'tunnel', label: 'TÚNEL'},
  {id: 'logs', label: 'LOGS'},
  {id: 'settings', label: 'CONFIG'},
];

// O useInput do Ink não distingue teclas F (chegam como input vazio), então
// escutamos o stdin cru e casamos as sequências clássicas de F1-F4.
const FKEY_SEQS = {
  '\x1bOP': 1, '\x1b[11~': 1, '\x1b[[A': 1,
  '\x1bOQ': 2, '\x1b[12~': 2, '\x1b[[B': 2,
  '\x1bOR': 3, '\x1b[13~': 3, '\x1b[[C': 3,
  '\x1bOS': 4, '\x1b[14~': 4, '\x1b[[D': 4,
};

function useFKeys(onFKey) {
  const {stdin} = useStdin();
  const ref = useRef(onFKey);
  ref.current = onFKey;
  useEffect(() => {
    if (!stdin) return;
    const onData = data => {
      const n = FKEY_SEQS[String(data)];
      if (n) ref.current(n);
    };
    stdin.on('data', onData);
    return () => { stdin.off('data', onData); };
  }, [stdin]);
}

function nowStamp() {
  return new Date().toTimeString().slice(0, 8);
}

// NavTabs: abas do cabeçalho (SESSÃO TÚNEL LOGS CONFIG) — a ativa invertida
// em âmbar, como o item ativo da nav do design.
function NavTabs({screen}) {
  return h(Text, null,
    ...SCREENS.map((s, i) => h(Text, {key: s.id},
      i > 0 ? h(Text, {color: theme.outlineDim}, ' ') : null,
      h(Text, {
        color: s.id === screen ? theme.onPrimary : theme.textDim,
        backgroundColor: s.id === screen ? theme.primary : undefined,
        bold: s.id === screen,
        dimColor: s.id !== screen,
      }, ` ${s.label} `),
    )),
  );
}

// Posições calculadas das abas — o mapeamento de cliques do mouse precisa
// concordar com o que NavTabs, a taskbar e o FilterTabs desenham.
function navTabAt(x, cols) {
  const total = SCREENS.reduce((t, s, i) => t + s.label.length + 2 + (i > 0 ? 1 : 0), 0);
  let cur = cols - total; // as abas terminam na coluna cols-1 (paddingX: 1)
  for (let i = 0; i < SCREENS.length; i++) {
    if (i > 0) cur += 1; // espaço separador
    const w = SCREENS[i].label.length + 2;
    if (x >= cur && x < cur + w) return SCREENS[i].id;
    cur += w;
  }
  return null;
}

function taskbarTabAt(x, screen) {
  const prefix = screen === 'session' ? 'SESSÃO_ATIVA' : 'MODO_DIAGNÓSTICO';
  let cur = 2 + prefix.length + 2;
  for (let i = 0; i < SCREENS.length; i++) {
    const w = `[F${i + 1}] ${SCREENS[i].label}`.length;
    if (x >= cur && x < cur + w) return SCREENS[i].id;
    cur += w + 1;
  }
  return null;
}

function filterTabAt(x) {
  let cur = 3; // borda do painel (1) + espaço inicial do FilterTabs
  for (let i = 0; i < FILTERS.length; i++) {
    const w = FILTERS[i].toUpperCase().replace(' ', '_').length;
    if (x >= cur && x < cur + w) return i;
    cur += w + 1; // separador │
  }
  return null;
}

export default function App({mouse}) {
  const {exit} = useApp();
  const {stdout} = useStdout();

  const bridge = useMemo(() => createBridge(), []);
  const [chats, setChats] = useState([]);
  const [msgs, setMsgs] = useState([]);
  const [log, setLog] = useState([{kind: 'text', text: 'iniciando o núcleo Go…', stamp: nowStamp()}]);
  const [status, setStatus] = useState({connected: false, lastSeen: ''});
  const [version, setVersion] = useState('');
  const [currentChat, setCurrentChat] = useState(null);
  const [playingId, setPlayingId] = useState('');
  const [screen, setScreen] = useState('session');
  const [focus, setFocus] = useState('chats');
  const [filter, setFilter] = useState(0);
  const [selChat, setSelChat] = useState(0);
  const [selMsg, setSelMsg] = useState(null);
  const [logScroll, setLogScroll] = useState(0);
  const [inputKey, setInputKey] = useState(0); // remonta o TextInput p/ limpar
  const [finderOpen, setFinderOpen] = useState(false);
  const [finderQuery, setFinderQuery] = useState('');
  const [finderScope, setFinderScope] = useState(0);
  const [finderSel, setFinderSel] = useState(0);
  const [finderKey, setFinderKey] = useState(0); // remonta o campo de busca p/ limpar

  const currentChatRef = useRef(null);
  currentChatRef.current = currentChat;
  const connRef = useRef(false); // detecta transição de conexão p/ logar no feed
  const mouseRef = useRef(null); // handler de mouse — atribuído mais abaixo

  useEffect(() => {
    if (!mouse) return;
    const fn = ev => mouseRef.current?.(ev);
    mouse.on('mouse', fn);
    return () => mouse.off('mouse', fn);
  }, [mouse]);

  useEffect(() => {
    const pushLog = (kind, text) =>
      setLog(l => [...l.slice(-300), {kind, text, stamp: nowStamp()}]);
    bridge.on('ready', e => setVersion(e.version || ''));
    bridge.on('chats', e => setChats(e.chats || []));
    bridge.on('screen', e => { setMsgs(e.messages || []); setSelMsg(null); });
    bridge.on('message', e => {
      if (e.message && currentChatRef.current && e.message.chatId === currentChatRef.current.id) {
        setMsgs(m => [...m, e.message]);
      }
    });
    bridge.on('status', e => {
      const connected = !!e.connected;
      if (connRef.current !== connected) {
        connRef.current = connected;
        pushLog('net', connected
          ? '[TÚNEL_ESTABELECIDO] conexão estável.'
          : '[TÚNEL_PERDIDO] aguardando reconexão…');
      }
      setStatus({connected, lastSeen: e.lastSeen || ''});
    });
    bridge.on('playing', e => setPlayingId(e.msgId || ''));
    bridge.on('text', e => pushLog('text', e.text));
    bridge.on('error', e => pushLog('error', e.text));
    bridge.on('file', e => pushLog('text', `arquivo salvo: ${e.path}`));
    bridge.on('exit', () => exit());
    bridge.start(); // listeners prontos: descarrega eventos chegados cedo
    return () => bridge.quit();
  }, [bridge, exit]);

  const visibleChats = useMemo(() => filterChats(chats, filter), [chats, filter]);

  // o finder busca em todas as conversas, ignorando o filtro da lista lateral
  const finderResults = useMemo(
    () => (finderOpen ? searchChats(chats, finderQuery, finderScope) : []),
    [finderOpen, chats, finderQuery, finderScope]);

  const openChat = chat => {
    if (!chat) return;
    setCurrentChat(chat);
    setMsgs([]);
    setSelMsg(null);
    bridge.send('select', [chat.id]);
  };

  const sendMessageCmd = cmd => {
    if (selMsg == null || !msgs[selMsg]) return;
    bridge.send(cmd, [msgs[selMsg].id]);
  };

  const openFinder = () => {
    setFinderOpen(true);
    setFinderQuery('');
    setFinderScope(0);
    setFinderSel(0);
    setFinderKey(k => k + 1); // limpa a busca anterior
  };

  // openFinderSelection abre o resultado destacado, sincroniza a seleção da
  // lista lateral (se a conversa estiver no filtro atual) e foca a digitação.
  const openFinderSelection = () => {
    const chat = finderResults[Math.max(0, Math.min(finderSel, finderResults.length - 1))];
    setFinderOpen(false);
    if (!chat) return;
    const idx = visibleChats.findIndex(c => c.id === chat.id);
    if (idx >= 0) setSelChat(idx);
    openChat(chat);
    setFocus('input');
  };

  useFKeys(n => {
    setScreen(SCREENS[n - 1].id);
    setLogScroll(0);
  });

  useInput((input, key) => {
    if (key.ctrl && input === 'q') { bridge.quit(); exit(); return; }
    if (screen !== 'session') {
      // fora da sessão: 1-4 também troca de tela; ↑/↓ rola os logs
      if (input >= '1' && input <= '4') { setScreen(SCREENS[Number(input) - 1].id); setLogScroll(0); return; }
      if (screen === 'logs') {
        if (key.upArrow) setLogScroll(s => Math.min(log.length, s + 1));
        else if (key.downArrow) setLogScroll(s => Math.max(0, s - 1));
        else if (key.pageUp) setLogScroll(s => Math.min(log.length, s + 10));
        else if (key.pageDown) setLogScroll(s => Math.max(0, s - 10));
      }
      if (key.escape) setScreen('session');
      return;
    }
    if (finderOpen) {
      // navegação do finder; Enter é tratado pelo onSubmit do campo de busca
      if (key.escape || (key.ctrl && input === 'f')) { setFinderOpen(false); return; }
      if (key.upArrow) { setFinderSel(s => Math.max(0, s - 1)); return; }
      if (key.downArrow) { setFinderSel(s => Math.min(Math.max(0, finderResults.length - 1), s + 1)); return; }
      if (key.tab) {
        const n = FINDER_SCOPES.length;
        setFinderScope(sc => (sc + (key.shift ? n - 1 : 1)) % n);
        setFinderSel(0);
        return;
      }
      return; // o resto das teclas vai para o campo de busca
    }
    if (key.ctrl && input === 'f') { openFinder(); return; }
    if (key.tab) {
      setFocus(f => FOCUS_ORDER[(FOCUS_ORDER.indexOf(f) + 1) % FOCUS_ORDER.length]);
      return;
    }
    if (focus === 'chats') {
      if (key.upArrow) setSelChat(s => Math.max(0, s - 1));
      else if (key.downArrow) setSelChat(s => Math.min(visibleChats.length - 1, s + 1));
      else if (key.return) { openChat(visibleChats[selChat]); setFocus('input'); }
      else if (input >= '1' && input <= '4') { setFilter(Number(input) - 1); setSelChat(0); }
      else if (input === 'f') { setFilter(f => (f + 1) % FILTERS.length); setSelChat(0); }
    } else if (focus === 'messages') {
      if (key.upArrow) setSelMsg(s => Math.max(0, (s == null ? msgs.length : s) - 1));
      else if (key.downArrow) setSelMsg(s => (s == null ? null : Math.min(msgs.length - 1, s + 1)));
      else if (key.escape) setSelMsg(null);
      else if (input === 'p' || key.return) sendMessageCmd('play');
      else if (input === 'o') sendMessageCmd('open');
      else if (input === 'd') sendMessageCmd('download');
      else if (input === 'b') bridge.send('backlog');
    }
  });

  const onSubmit = value => {
    const text = value.trim();
    setInputKey(k => k + 1); // limpa o campo
    if (!text) return;
    if (text.startsWith('/')) {
      const [cmd, ...params] = text.slice(1).split(' ');
      if (cmd === 'quit') { bridge.quit(); exit(); return; }
      bridge.send(cmd, params);
      return;
    }
    if (!currentChat) return;
    bridge.send('send', [currentChat.id, text]);
  };

  const rows = stdout?.rows || 30;
  const cols = stdout?.columns || 80;
  const innerHeight = rows - 4; // cabeçalho (2) + taskbar (2)
  const sessionHeight = innerHeight - 4; // prompt (3) + linha de dicas (1)

  // mouse: o emitter vem de index.mjs (stdin filtrado); o handler vive num
  // ref reatribuído a cada render para enxergar sempre o estado atual
  mouseRef.current = ({type, x, y, dy}) => {
    if (type === 'click' && y === 1) {
      const id = navTabAt(x, cols);
      if (id) { setScreen(id); setLogScroll(0); }
      return;
    }
    if (type === 'click' && y === rows) {
      const id = taskbarTabAt(x, screen);
      if (id) { setScreen(id); setLogScroll(0); }
      return;
    }
    if (screen === 'logs' && type === 'wheel') {
      setLogScroll(s => Math.max(0, Math.min(log.length, s - dy * 3)));
      return;
    }
    if (screen !== 'session') return;
    if (finderOpen) {
      // com o finder aberto, a roda navega os resultados; cliques ficam de fora
      if (type === 'wheel') {
        setFinderSel(s => Math.max(0, Math.min(Math.max(0, finderResults.length - 1), s + dy)));
      }
      return;
    }
    const bodyTop = 3; // linha 1 = cabeçalho, linha 2 = borda
    const bodyBottom = 2 + sessionHeight;
    if (y >= bodyTop && y <= bodyBottom) {
      if (x <= SIDEBAR_WIDTH) {
        if (type === 'wheel') {
          setFocus('chats');
          setSelChat(s => Math.max(0, Math.min(visibleChats.length - 1, s + dy)));
          return;
        }
        if (y === bodyTop + 3) { // linha das abas de filtro
          const f = filterTabAt(x);
          if (f != null) { setFilter(f); setSelChat(0); }
          return;
        }
        // linhas de conversa: borda + título + peers + filtros = 4 linhas;
        // a janela de rolagem é a mesma que o ChatList desenha
        const list = chatRows(visibleChats);
        const visible = Math.max(1, sessionHeight - 5);
        const selRow = Math.max(0, list.findIndex(r => r.index === selChat));
        const start = rowScrollStart(list.length, selRow, visible);
        const row = y >= bodyTop + 4 ? list[start + (y - bodyTop - 4)] : null;
        if (row && row.chat) {
          setSelChat(row.index);
          openChat(row.chat);
          setFocus('input');
        }
        return;
      }
      // painel de mensagens: clique foca, roda navega como ↑/↓
      setFocus('messages');
      if (type === 'wheel') {
        if (dy < 0) setSelMsg(s => Math.max(0, (s == null ? msgs.length : s) - 1));
        else setSelMsg(s => (s == null ? null : Math.min(msgs.length - 1, s + 1)));
      }
      return;
    }
    // caixa do prompt (3 linhas logo abaixo dos painéis)
    if (type === 'click' && y > bodyBottom && y <= bodyBottom + 3) setFocus('input');
  };

  let body;
  if (screen === 'tunnel') {
    body = h(TunnelScreen, {status, chats, version, log, height: innerHeight});
  } else if (screen === 'logs') {
    body = h(LogsScreen, {log, status, scroll: logScroll, height: innerHeight});
  } else if (screen === 'settings') {
    body = h(SettingsScreen, {version, status, binPath: bridge.bin, height: innerHeight});
  } else {
    body = h(Box, {flexDirection: 'column', height: innerHeight},
      h(Box, {flexGrow: 1},
        finderOpen
          ? h(Finder, {
            results: finderResults,
            scope: finderScope,
            selected: finderSel,
            height: sessionHeight,
            width: Math.max(30, Math.min(72, cols - 8)),
            inputKey: finderKey,
            onChange: v => { setFinderQuery(v); setFinderSel(0); },
            onSubmit: openFinderSelection,
          })
          : h(React.Fragment, null,
            h(ChatList, {
              chats: visibleChats,
              filter,
              selected: selChat,
              currentId: currentChat?.id,
              focused: focus === 'chats',
              height: sessionHeight,
              width: SIDEBAR_WIDTH,
            }),
            h(Messages, {
              msgs,
              log,
              chatName: currentChat?.name,
              selected: selMsg,
              playingId,
              focused: focus === 'messages',
              height: sessionHeight,
            }),
          ),
      ),
      // prompt de entrada
      h(Box, {
        borderStyle: 'single',
        borderColor: focus === 'input' ? theme.primary : theme.outlineDim,
        paddingX: 1,
      },
        h(Text, {color: theme.primary, bold: true}, 'você@zapterm:~$ '),
        h(TextInput, {
          key: inputKey,
          isDisabled: focus !== 'input' || finderOpen,
          placeholder: 'digite_mensagem_ou_comando…',
          onSubmit,
        }),
      ),
      h(Box, {paddingX: 1},
        h(Text, {color: theme.textDim, dimColor: true, wrap: 'truncate'},
          '[TAB] painel · [↑/↓] navegar · [ENTER] abrir/enviar · [CTRL+F] buscar · [1-4] filtros · [P] áudio · [O] abrir · [D] baixar · [B] histórico · mouse: clique/rolagem'),
      ),
    );
  }

  return h(Box, {flexDirection: 'column', height: rows},
    // cabeçalho: título do protocolo + abas de navegação
    h(Box, {
      justifyContent: 'space-between',
      paddingX: 1,
      borderStyle: 'single',
      borderColor: theme.outlineDim,
      borderTop: false, borderLeft: false, borderRight: false,
    },
      h(Text, {wrap: 'truncate'},
        h(Text, {color: theme.primary, bold: true}, 'ZAPTERM PROTOCOL'),
        h(Text, {color: theme.secondary}, version ? ` ${version}` : ''),
        h(Text, {color: theme.outlineDim}, ' — '),
        h(Text, {color: status.connected ? theme.textDim : theme.error},
          status.connected ? 'CONEXÃO CRIPTOGRAFADA' : 'SEM CONEXÃO'),
      ),
      h(NavTabs, {screen}),
    ),
    body,
    // taskbar
    h(Box, {
      justifyContent: 'space-between',
      paddingX: 1,
      borderStyle: 'single',
      borderColor: theme.outlineDim,
      borderBottom: false, borderLeft: false, borderRight: false,
    },
      h(Text, {wrap: 'truncate'},
        h(Text, {color: theme.primaryDim}, screen === 'session' ? 'SESSÃO_ATIVA' : 'MODO_DIAGNÓSTICO'),
        h(Text, null, '  '),
        ...SCREENS.map((s, i) => h(Text, {key: s.id},
          h(Text, {
            color: s.id === screen ? theme.onPrimary : theme.secondary,
            backgroundColor: s.id === screen ? theme.primary : undefined,
          }, `[F${i + 1}] ${s.label}`),
          i < SCREENS.length - 1 ? h(Text, null, ' ') : null,
        )),
      ),
      h(Text, null,
        h(Text, {color: status.connected ? theme.tertiary : theme.error, bold: true},
          status.connected ? '[ONLINE]' : '[OFFLINE]'),
        h(Text, {color: theme.error}, '  [CTRL+Q] SAIR'),
      ),
    ),
  );
}
