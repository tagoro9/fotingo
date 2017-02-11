'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _fs = require('fs');

var _fs2 = _interopRequireDefault(_fs);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _configFilePath = require('./config-file-path');

var _configFilePath2 = _interopRequireDefault(_configFilePath);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var createEmptyConfigFile = _ramda2['default'].curryN(3, _fs2['default'].writeFileSync)(_ramda2['default'].__, _ramda2['default'].__, 'utf8');
var readConfigFile = _ramda2['default'].curryN(2, _ramda2['default'].compose(JSON.parse, _fs2['default'].readFileSync))(_ramda2['default'].__, 'utf8');

// export default R.compose(JSON.parse, R.tryCatch(readUtf8File, curriedWriteFileSync, {}))(filePath);


exports['default'] = function (defaults) {
  try {
    return readConfigFile(_configFilePath2['default']);
  } catch (e) {
    createEmptyConfigFile(_configFilePath2['default'], JSON.stringify(defaults));
    return defaults;
  }
};