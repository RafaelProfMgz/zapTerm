// Testes unitários das funções puras da UI (filtros, busca fuzzy, stories).
// Não renderizam nada — só verificam a lógica que alimenta os componentes.
import {test} from 'node:test';
import assert from 'node:assert/strict';

import {filterChats, sortChats, chatRows, rowScrollStart, shortTime} from '../src/chatlist.mjs';
import {fuzzyMatch, searchChats, rawJid} from '../src/finder.mjs';
import {kindLabel, storyPreview} from '../src/stories.mjs';

const CHATS = [
  {id: 'a@s.whatsapp.net', isGroup: false, name: 'Alice', unread: 2, lastMessage: 300},
  {id: 'g@g.us', isGroup: true, name: 'Equipe', unread: 0, lastMessage: 200},
  {id: 'b@s.whatsapp.net', isGroup: false, name: 'Bob', unread: 0, lastMessage: 0},
];

test('sortChats: mais recentes primeiro, sem histórico por último', () => {
  const sorted = sortChats(CHATS);
  assert.deepEqual(sorted.map(c => c.name), ['Alice', 'Equipe', 'Bob']);
});

test('filterChats: filtros Todas/Não lidas/Grupos/Contatos', () => {
  assert.equal(filterChats(CHATS, 0).length, 3);
  assert.deepEqual(filterChats(CHATS, 1).map(c => c.name), ['Alice']);
  assert.deepEqual(filterChats(CHATS, 2).map(c => c.name), ['Equipe']);
  assert.deepEqual(filterChats(CHATS, 3).map(c => c.name), ['Alice', 'Bob']);
});

test('chatRows: separador entre conversas com e sem histórico', () => {
  const rows = chatRows(sortChats(CHATS));
  const sepCount = rows.filter(r => r.sep).length;
  assert.equal(sepCount, 1, 'deve haver 1 separador SEM_HISTÓRICO');
  // o separador vem antes do Bob (lastMessage 0)
  const sepIdx = rows.findIndex(r => r.sep);
  assert.equal(rows[sepIdx + 1].chat.name, 'Bob');
});

test('rowScrollStart: janela centrada e dentro dos limites', () => {
  assert.equal(rowScrollStart(100, 0, 10), 0);
  assert.equal(rowScrollStart(100, 50, 10), 45);
  assert.equal(rowScrollStart(100, 99, 10), 90);
  assert.equal(rowScrollStart(3, 0, 10), 0, 'lista menor que a janela começa em 0');
});

test('shortTime: hoje vira HH:MM', () => {
  const now = Math.floor(Date.now() / 1000);
  assert.match(shortTime(now), /^\d{2}:\d{2}$/);
});

test('fuzzyMatch: subsequência case-insensitive', () => {
  assert.equal(fuzzyMatch('abc', 'aXbXc').ok, true);
  assert.equal(fuzzyMatch('abc', 'acb').ok, false);
  assert.equal(fuzzyMatch('', 'qualquer').ok, true, 'busca vazia casa com tudo');
  // match no começo pontua mais que no meio
  assert.ok(fuzzyMatch('al', 'alice').score > fuzzyMatch('al', 'realmadrid').score);
});

test('searchChats: respeita escopo e ordena por relevância', () => {
  const all = searchChats(CHATS, 'b', 0).map(c => c.name);
  assert.ok(all.includes('Bob'));
  const onlyGroups = searchChats(CHATS, '', 2).map(c => c.name);
  assert.deepEqual(onlyGroups, ['Equipe']);
  const onlyContacts = searchChats(CHATS, '', 1).map(c => c.name).sort();
  assert.deepEqual(onlyContacts, ['Alice', 'Bob']);
});

test('rawJid: tira o sufixo do WhatsApp', () => {
  assert.equal(rawJid('5511999@s.whatsapp.net'), '5511999');
  assert.equal(rawJid('xpto@g.us'), 'xpto');
});

test('kindLabel: rótulos pt-BR por tipo de mídia', () => {
  assert.equal(kindLabel('image'), '[imagem]');
  assert.equal(kindLabel('video'), '[vídeo]');
  assert.equal(kindLabel('text'), '');
});

test('storyPreview: combina rótulo de mídia e legenda', () => {
  assert.equal(storyPreview({kind: 'image', text: 'praia'}), '[imagem] praia');
  assert.equal(storyPreview({kind: 'image', text: ''}), '[imagem]');
  assert.equal(storyPreview({kind: 'text', text: 'oi'}), 'oi');
  assert.equal(storyPreview({kind: 'text', text: ''}), '(sem legenda)');
});
