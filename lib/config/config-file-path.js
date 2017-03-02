'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.localConfigFilePath = exports.globalConfigFilePath = exports.CONFIG_FILE_NAME = undefined;

var _path = require('path');

var _path2 = _interopRequireDefault(_path);

var _package = require('../../package.json');

var _package2 = _interopRequireDefault(_package);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var CONFIG_FILE_NAME = exports.CONFIG_FILE_NAME = '.' + _package2['default'].name;

var globalConfigFilePath = exports.globalConfigFilePath = _path2['default'].resolve(process.env.HOME || process.env.USERPROFILE, CONFIG_FILE_NAME);
var localConfigFilePath = exports.localConfigFilePath = _path2['default'].resolve(process.cwd(), CONFIG_FILE_NAME);