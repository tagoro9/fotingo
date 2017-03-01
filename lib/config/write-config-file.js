'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _fs = require('fs');

var _fs2 = _interopRequireDefault(_fs);

var _configFilePath = require('./config-file-path');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

exports['default'] = function (data) {
  var local = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : false;
  return _fs2['default'].writeFileSync(local ? _configFilePath.localConfigFilePath : _configFilePath.globalConfigFilePath, JSON.stringify(data, null, 2), { encoding: 'utf8', flag: 'w' });
};