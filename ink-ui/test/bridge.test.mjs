// e2e do bridge: sobe um "núcleo" falso (test-fixtures/fake-core.mjs) que fala
// o mesmo protocolo NDJSON do binário Go (--ui=json) e verifica o handshake,
// o round-trip de comando e o encerramento limpo — sem tocar no WhatsApp.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
process.env.ZAPTERM_BIN = path.join(here, '..', 'test-fixtures', 'fake-core.mjs');

const {createBridge} = await import('../src/bridge.mjs');

function waitFor(bridge, type, timeout = 4000) {
  return new Promise((resolve, reject) => {
    const t = setTimeout(() => reject(new Error('timeout esperando evento: ' + type)), timeout);
    bridge.on(type, e => {
      clearTimeout(t);
      resolve(e);
    });
  });
}

test('bridge: handshake ready/chats/stories', async () => {
  const bridge = createBridge();
  const readyP = waitFor(bridge, 'ready');
  const chatsP = waitFor(bridge, 'chats');
  const storiesP = waitFor(bridge, 'stories');
  bridge.start();

  const ready = await readyP;
  assert.equal(ready.version, 'vTEST');

  const chats = await chatsP;
  assert.equal(chats.chats.length, 2);
  assert.equal(chats.chats[0].name, 'Alice');

  const stories = await storiesP;
  assert.equal(stories.stories.length, 1);
  assert.equal(stories.stories[0].name, 'Bob');
  assert.equal(stories.stories[0].messages[0].kind, 'image');

  const exitP = waitFor(bridge, 'exit');
  bridge.quit();
  await exitP;
});

test('bridge: comando select devolve a tela de mensagens', async () => {
  const bridge = createBridge();
  bridge.start();
  await waitFor(bridge, 'ready');

  const screenP = waitFor(bridge, 'screen');
  bridge.send('select', ['123@s.whatsapp.net']);
  const screen = await screenP;
  assert.equal(screen.messages[0].chatId, '123@s.whatsapp.net');
  assert.equal(screen.messages[0].text, 'oi');

  const exitP = waitFor(bridge, 'exit');
  bridge.quit();
  await exitP;
});

test('bridge: /reconectar traz o QR de pareamento e /cancelqr o encerra', async () => {
  const bridge = createBridge();
  bridge.start();
  await waitFor(bridge, 'ready');

  const qrP = waitFor(bridge, 'qr');
  bridge.send('reconectar');
  const qr = await qrP;
  assert.equal(qr.event, 'code');
  assert.equal(qr.matrix.length, 65, 'matriz do tamanho de um código real');
  assert.equal(qr.matrix[0].length, 65);
  assert.ok(qr.png);

  const doneP = waitFor(bridge, 'qr');
  bridge.send('cancelqr');
  const done = await doneP;
  assert.equal(done.event, 'done');

  const exitP = waitFor(bridge, 'exit');
  bridge.quit();
  await exitP;
});

test('bridge: eventos chegados antes do start() são bufferizados', async () => {
  const bridge = createBridge();
  // sem chamar start() imediatamente: dá tempo de o fake-core emitir
  await new Promise(r => setTimeout(r, 150));
  const readyP = waitFor(bridge, 'ready');
  bridge.start(); // descarrega o buffer p/ os listeners
  const ready = await readyP;
  assert.equal(ready.version, 'vTEST');

  const exitP = waitFor(bridge, 'exit');
  bridge.quit();
  await exitP;
});

test('bridge: comando com account só mexe naquela conta', async () => {
  const bridge = createBridge();
  bridge.start();
  const accounts = await waitFor(bridge, 'accounts');
  assert.equal(accounts.active, 'default');
  assert.deepEqual(accounts.accounts.map(a => a.id), ['default', 'trabalho']);

  const screens = [];
  bridge.on('screen', e => screens.push(e));
  const echoP = waitFor(bridge, 'text');
  bridge.send('select', ['999@s.whatsapp.net'], 'trabalho');
  const echo = await echoP;
  assert.equal(echo.account, 'trabalho');
  assert.equal(screens.length, 1);
  assert.equal(screens[0].account, 'trabalho', 'a tela veio da conta pedida');

  // sem account: vai para a conta ativa
  const activeP = waitFor(bridge, 'screen');
  bridge.send('select', ['123@s.whatsapp.net']);
  assert.equal((await activeP).account, 'default');

  const exitP = waitFor(bridge, 'exit');
  bridge.quit();
  await exitP;
});

test('bridge: /conta troca a conta ativa e reenvia os chats', async () => {
  const bridge = createBridge();
  bridge.start();
  await waitFor(bridge, 'ready');

  // o núcleo anuncia a conta ativa na subida; aqui interessa só a troca
  const switchP = new Promise(resolve => bridge.on('account', e => { if (e.id === 'trabalho') resolve(e); }));
  const chatsP = new Promise(resolve => bridge.on('chats', e => { if (e.account === 'trabalho') resolve(e); }));
  bridge.send('conta', ['trabalho']);
  assert.equal((await switchP).id, 'trabalho');
  const chats = await chatsP;
  assert.equal(chats.chats[0].name, 'Cliente');

  const exitP = waitFor(bridge, 'exit');
  bridge.quit();
  await exitP;
});
