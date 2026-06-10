import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Box, Text, useApp, useInput, useStdout} from 'ink';
import {TextInput} from '@inkjs/ui';
import {createBridge} from './bridge.mjs';
import theme from './theme.mjs';
import ChatList, {FILTERS, filterChats} from './chatlist.mjs';
import Messages from './messages.mjs';

const h = React.createElement;

const SIDEBAR_WIDTH = 34;
const FOCUS_ORDER = ['chats', 'messages', 'input'];

export default function App() {
  const {exit} = useApp();
  const {stdout} = useStdout();

  const bridge = useMemo(() => createBridge(), []);
  const [chats, setChats] = useState([]);
  const [msgs, setMsgs] = useState([]);
  const [log, setLog] = useState([{kind: 'text', text: 'iniciando o núcleo Go…'}]);
  const [status, setStatus] = useState({connected: false, lastSeen: ''});
  const [currentChat, setCurrentChat] = useState(null);
  const [playingId, setPlayingId] = useState('');
  const [focus, setFocus] = useState('chats');
  const [filter, setFilter] = useState(0);
  const [selChat, setSelChat] = useState(0);
  const [selMsg, setSelMsg] = useState(null);
  const [inputKey, setInputKey] = useState(0); // remonta o TextInput p/ limpar

  const currentChatRef = useRef(null);
  currentChatRef.current = currentChat;

  useEffect(() => {
    const pushLog = (kind, text) =>
      setLog(l => [...l.slice(-300), {kind, text}]);
    bridge.on('chats', e => setChats(e.chats || []));
    bridge.on('screen', e => { setMsgs(e.messages || []); setSelMsg(null); });
    bridge.on('message', e => {
      if (e.message && currentChatRef.current && e.message.chatId === currentChatRef.current.id) {
        setMsgs(m => [...m, e.message]);
      }
    });
    bridge.on('status', e => setStatus({connected: !!e.connected, lastSeen: e.lastSeen || ''}));
    bridge.on('playing', e => setPlayingId(e.msgId || ''));
    bridge.on('text', e => pushLog('text', e.text));
    bridge.on('error', e => pushLog('error', e.text));
    bridge.on('file', e => pushLog('text', `arquivo salvo: ${e.path}`));
    bridge.on('exit', () => exit());
    return () => bridge.quit();
  }, [bridge, exit]);

  const visibleChats = useMemo(() => filterChats(chats, filter), [chats, filter]);

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

  useInput((input, key) => {
    if (key.ctrl && input === 'q') { bridge.quit(); exit(); return; }
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
  const innerHeight = rows - 5; // header + input + hint

  return h(Box, {flexDirection: 'column', height: rows},
    // header
    h(Box, {justifyContent: 'space-between', paddingX: 1},
      h(Text, null,
        h(Text, {color: theme.accent, bold: true}, 'ZAPTERM'),
        h(Text, {color: theme.clay}, ' — ink ui experimental · núcleo Go'),
      ),
      h(Text, {color: status.connected ? theme.accent : theme.danger, bold: true},
        status.connected ? '[ONLINE]' : '[OFFLINE]'),
    ),
    // corpo: sidebar + mensagens
    h(Box, {flexGrow: 1, height: innerHeight},
      h(ChatList, {
        chats: visibleChats,
        filter,
        selected: selChat,
        currentId: currentChat?.id,
        focused: focus === 'chats',
        height: innerHeight,
        width: SIDEBAR_WIDTH,
      }),
      h(Messages, {
        msgs,
        log,
        chatName: currentChat?.name,
        selected: selMsg,
        playingId,
        focused: focus === 'messages',
        height: innerHeight,
      }),
    ),
    // input estilo prompt
    h(Box, {borderStyle: 'single', borderColor: focus === 'input' ? theme.accent : theme.clayDark, paddingX: 1},
      h(Text, {color: theme.green, bold: true}, 'você@zapterm:~$ '),
      h(TextInput, {
        key: inputKey,
        isDisabled: focus !== 'input',
        placeholder: 'digite_mensagem_ou_comando…',
        onSubmit,
      }),
    ),
    // hint bar
    h(Box, {paddingX: 1},
      h(Text, {color: theme.clay},
        h(Text, {color: theme.danger, bold: true}, '[CTRL+Q]'), ' sair  ',
        h(Text, {color: theme.sage, bold: true}, '[TAB]'), ' painel  ',
        h(Text, {color: theme.sage, bold: true}, '[1-4]'), ' filtrar  ',
        h(Text, {color: theme.sage, bold: true}, '[↑/↓]'), ' navegar  ',
        h(Text, {color: theme.sage, bold: true}, '[P]'), ' áudio  ',
        h(Text, {color: theme.sage, bold: true}, '[B]'), ' histórico',
      ),
    ),
  );
}
