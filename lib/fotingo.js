#!/usr/bin/env node
'use strict';

var _commander = require('commander');

var _commander2 = _interopRequireDefault(_commander);

var _package = require('../package.json');

var _package2 = _interopRequireDefault(_package);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

_commander2['default'].version(_package2['default'].version).command('start [issue-id]', 'start working in a new issue').command('review', 'submit current issue for review').parse(process.argv);