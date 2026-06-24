#!/usr/bin/env node
// fake-core.mjs — núcleo Go falso para os testes e2e do bridge: fala o mesmo
// protocolo NDJSON (--ui=json) sem tocar no WhatsApp. Emite ready/chats/
// stories na subida e responde a comandos pelo stdin.
import {createInterface} from 'node:readline';

const emit = o => process.stdout.write(JSON.stringify(o) + '\n');

emit({type: 'ready', version: 'vTEST'});
emit({type: 'status', connected: true, lastSeen: ''});
emit({
  type: 'chats',
  chats: [
    {id: '123@s.whatsapp.net', isGroup: false, name: 'Alice', unread: 2, lastMessage: 1000},
    {id: 'grp@g.us', isGroup: true, name: 'Equipe', unread: 0, lastMessage: 900},
  ],
});
emit({
  type: 'stories',
  stories: [
    {
      senderId: '555@s.whatsapp.net',
      name: 'Bob',
      short: 'Bob',
      unread: 1,
      lastMessage: 2000,
      messages: [{id: 's1', chatId: 'status@broadcast', kind: 'image', text: 'praia', timestamp: 2000, fromMe: false}],
    },
  ],
});

const rl = createInterface({input: process.stdin});
rl.on('line', line => {
  let cmd;
  try {
    cmd = JSON.parse(line);
  } catch {
    return;
  }
  if (cmd.cmd === 'quit') {
    process.exit(0);
  }
  if (cmd.cmd === 'select') {
    emit({
      type: 'screen',
      messages: [{id: 'm1', chatId: cmd.params[0], text: 'oi', kind: 'text', timestamp: 1000, fromMe: false}],
    });
  }
  emit({type: 'text', text: 'recv:' + cmd.cmd}); // eco p/ asserções
});
