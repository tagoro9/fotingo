'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _fs = require('fs');

var _fs2 = _interopRequireDefault(_fs);

var _configFilePath = require('./config-file-path');

var _configFilePath2 = _interopRequireDefault(_configFilePath);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

exports['default'] = function (data) {
  return _fs2['default'].writeFileSync(_configFilePath2['default'], JSON.stringify(data, null, 2), { encoding: 'utf8', flag: 'w' });
};