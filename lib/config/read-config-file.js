'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _fs = require('fs');

var _fs2 = _interopRequireDefault(_fs);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _configFilePath = require('./config-file-path');

var _error = require('../error');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var createEmptyConfigFile = _ramda2['default'].curryN(3, _fs2['default'].writeFileSync)(_ramda2['default'].__, _ramda2['default'].__, 'utf8');
var readConfigFile = _ramda2['default'].curryN(2, _ramda2['default'].compose(JSON.parse, _fs2['default'].readFileSync))(_ramda2['default'].__, 'utf8');

var getGlobalConfig = _ramda2['default'].tryCatch(_ramda2['default'].compose(readConfigFile, _ramda2['default'].always(_configFilePath.globalConfigFilePath)), _ramda2['default'].ifElse(_ramda2['default'].propEq('code', 'ENOENT'), _ramda2['default'].converge(_ramda2['default'].identity, [_ramda2['default'].nthArg(1), _ramda2['default'].compose(createEmptyConfigFile(_configFilePath.globalConfigFilePath), function (defaults) {
  return JSON.stringify(defaults, null, 2);
}, _ramda2['default'].nthArg(1))]), function () {
  return (0, _error.handleError)(new _error.ControlledError(_error.errors.config.malformedFile));
}));

var getLocalConfig = _ramda2['default'].tryCatch(_ramda2['default'].compose(_ramda2['default'].set(_ramda2['default'].lensProp('local'), true), readConfigFile, _ramda2['default'].always(_configFilePath.localConfigFilePath)), _ramda2['default'].ifElse(_ramda2['default'].propEq('code', 'ENOENT'), _ramda2['default'].always({}), function () {
  return (0, _error.handleError)(new _error.ControlledError(_error.errors.config.malformedFile));
}));

exports['default'] = _ramda2['default'].converge(_ramda2['default'].mergeWith(_ramda2['default'].ifElse(_ramda2['default'].is(Object), _ramda2['default'].merge, _ramda2['default'].nthArg(1))), [getGlobalConfig, getLocalConfig]);