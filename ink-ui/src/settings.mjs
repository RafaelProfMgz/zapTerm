import React from 'react';
import {Box, Text} from 'ink';
import theme from './theme.mjs';

const h = React.createElement;

// Card: cartão do design de settings — título em caps separado do corpo,
// contorno de 1px, sem sombra.
function Card({title, children, grow}) {
  return h(Box, {
    flexDirection: 'column',
    flexGrow: grow ? 1 : 0,
    flexBasis: grow ? 0 : undefined,
    overflow: 'hidden',
    borderStyle: 'single',
    borderColor: theme.outlineDim,
    paddingX: 1,
    marginRight: 1,
  },
    h(Text, {color: theme.primary, bold: true}, `[${title}]`),
    ...children,
  );
}

function Field({label, value, color}) {
  return h(Text, {wrap: 'truncate'},
    h(Text, {color: theme.textDim, dimColor: true}, `${label}: `),
    h(Text, {color: color || theme.text}, value),
  );
}

function Key({k, action}) {
  return h(Text, {wrap: 'truncate'},
    h(Text, {color: theme.secondary, bold: true}, `[${k}]`.padEnd(10)),
    h(Text, {color: theme.textDim}, action),
  );
}

// Swatch: amostras do tema, como os "THEME_CORES" do design.
function Swatch({colors}) {
  return h(Text, null,
    ...colors.map((c, i) => h(Text, {key: i, backgroundColor: c}, '  ')),
  );
}

export default function SettingsScreen({version, status, binPath, height}) {
  return h(Box, {flexDirection: 'column', height, overflow: 'hidden', paddingX: 1},
    h(Box, {justifyContent: 'space-between'},
      h(Text, {color: theme.primary, bold: true}, 'CONFIG_DO_SISTEMA'),
      h(Text, {color: theme.textDim, dimColor: true}, 'somente leitura nesta tela'),
    ),
    h(Text, {color: theme.textDim, dimColor: true},
      'Interface de configuração do ZapTerm. Edite o INI abaixo e reinicie.'),
    h(Text, null, ' '),
    h(Box, null,
      h(Card, {title: 'ARQUIVOS_DO_SISTEMA', grow: true, children: [
        h(Field, {key: 'cfg', label: 'CONFIG ', value: '~/.config/whatscli/whatscli.config'}),
        h(Field, {key: 'db', label: 'SESSÃO ', value: '~/.config/whatscli/session.db'}),
      ]}),
      h(Card, {title: 'NÚCLEO_GO', grow: true, children: [
        h(Field, {key: 'bin', label: 'BINÁRIO', value: binPath || '?'}),
        h(Field, {key: 'ver', label: 'VERSÃO ', value: version || '—', color: theme.secondary}),
        h(Field, {
          key: 'st',
          label: 'ESTADO ',
          value: status.connected ? 'CONECTADO' : 'DESCONECTADO',
          color: status.connected ? theme.primary : theme.error,
        }),
      ]}),
    ),
    h(Box, null,
      h(Card, {title: 'NÚCLEOS_DE_TEMA', grow: true, children: [
        h(Text, {key: 'n', color: theme.text}, 'TERMINAL_PROTOCOL — âmbar sobre terra'),
        h(Box, {key: 'sw'},
          h(Swatch, {colors: [theme.primary, theme.primaryDim, theme.secondary, theme.tertiary, theme.text, theme.surfaceHighest]}),
          h(Text, {color: theme.primary, bold: true}, '  ● ATIVO'),
        ),
      ]}),
      h(Card, {title: 'FRONTEND', grow: true, children: [
        h(Field, {key: 'ui', label: 'UI     ', value: 'ink (experimental)'}),
        h(Field, {key: 'ponte', label: 'PONTE  ', value: 'NDJSON via stdio', color: theme.tertiary}),
      ]}),
    ),
    h(Box, {flexGrow: 1},
      h(Card, {title: 'ATALHOS', grow: true, children: [
        h(Key, {key: 'f', k: 'F1-F4', action: 'telas: sessão · túnel · logs · config'}),
        h(Key, {key: 'tab', k: 'TAB', action: 'alterna painéis (conversas → mensagens → digitação)'}),
        h(Key, {key: 'nav', k: '↑/↓', action: 'navegar / rolar logs'}),
        h(Key, {key: 'ent', k: 'ENTER', action: 'abrir conversa · enviar mensagem · tocar áudio'}),
        h(Key, {key: 'find', k: 'CTRL+F', action: 'buscar contato por nome ou número (TAB alterna escopo)'}),
        h(Key, {key: 'fil', k: '1-4 / F', action: 'filtros de conversa: todas · não lidas · grupos · contatos'}),
        h(Key, {key: 'pod', k: 'P/O/D', action: 'tocar áudio · abrir anexo · baixar (mensagem selecionada)'}),
        h(Key, {key: 'b', k: 'B', action: 'carregar histórico (backlog)'}),
        h(Key, {key: 'cmd', k: '/CMD', action: 'comando do núcleo Go (/connect, /read, /sendaudio…)'}),
        h(Key, {key: 'q', k: 'CTRL+Q', action: 'sair'}),
      ]}),
    ),
  );
}
