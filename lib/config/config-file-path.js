'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _path = require('path');

var _path2 = _interopRequireDefault(_path);

var _package = require('../../package.json');

var _package2 = _interopRequireDefault(_package);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var CONFIG_FILE_NAME = '.' + _package2['default'].name;

exports['default'] = _path2['default'].resolve(process.env.HOME || process.env.USERPROFILE, CONFIG_FILE_NAME);