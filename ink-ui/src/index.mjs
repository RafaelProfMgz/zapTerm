#!/usr/bin/env node
import React from 'react';
import {render} from 'ink';
import App from './app.mjs';
import {createMouseStdin, enableMouse, disableMouse} from './mouse.mjs';

// stdin filtrado: eventos de mouse viram `mouse` (emitter); o resto vai ao Ink
const {stdin, events} = createMouseStdin(process.stdin);
enableMouse();
process.on('exit', () => disableMouse());

const app = render(React.createElement(App, {mouse: events}), {stdin});
app.waitUntilExit().finally(() => disableMouse());
