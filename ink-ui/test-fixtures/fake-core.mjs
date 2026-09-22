#!/usr/bin/env node
// fake-core.mjs — núcleo Go falso para os testes e2e do bridge: fala o mesmo
// protocolo NDJSON (--ui=json) sem tocar no WhatsApp. Emite ready/chats/
// stories na subida e responde a comandos pelo stdin.
import {createInterface} from 'node:readline';

const emit = o => process.stdout.write(JSON.stringify(o) + '\n');

// duas contas, como o núcleo real com accounts.json: todo evento de sessão
// leva "account"; comandos sem "account" vão para a conta ativa
const accounts = [
  {id: 'default', label: 'Pessoal', jid: '5511900000001@s.whatsapp.net', connected: true, loggedIn: true},
  {id: 'trabalho', label: 'Trabalho', jid: '', connected: false, loggedIn: false, needsLogin: true},
];
let active = 'default';
const chatsOf = {
  trabalho: [{id: '999@s.whatsapp.net', isGroup: false, name: 'Cliente', unread: 1, lastMessage: 3000}],
};

emit({type: 'ready', version: 'vTEST'});
emit({type: 'account', id: active});
emit({type: 'accounts', active, accounts});
emit({type: 'status', account: 'default', connected: true, lastSeen: ''});
emit({
  type: 'chats',
  account: 'default',
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
  const account = cmd.account || active;
  if (cmd.cmd === 'conta' && accounts.some(a => a.id === cmd.params?.[0])) {
    active = cmd.params[0];
    emit({type: 'account', id: active});
    emit({type: 'accounts', active, accounts});
    emit({type: 'chats', account: active, chats: chatsOf[active] || []});
  }
  if (cmd.cmd === 'reconectar' || cmd.cmd === 'novoqr') {
    emit({type: 'status', connected: false, loggedIn: false, connecting: true, needsLogin: false});
    emit({
      type: 'qr',
      event: 'code',
      code: 'fake-pair-code',
      // matriz 65x65, do tamanho de um código de pareamento real (~277 chars)
      matrix: Array.from({length: 65}, (_, y) =>
        Array.from({length: 65}, (_, x) => ((x * 7 + y * 13) % 3 ? '1' : '0')).join('')),
      png: '/tmp/whatscli-qr.png',
      message: 'leia o QR code no celular',
    });
  }
  if (cmd.cmd === 'cancelqr') {
    emit({type: 'qr', event: 'done', message: 'leitura do QR code cancelada'});
  }
  if (cmd.cmd === 'select') {
    emit({
      type: 'screen',
      account,
      messages: [{id: 'm1', chatId: cmd.params[0], text: 'oi', kind: 'text', timestamp: 1000, fromMe: false}],
    });
  }
  emit({type: 'text', account, text: 'recv:' + cmd.cmd}); // eco p/ asserções
});
