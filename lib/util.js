'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.promisify = exports.wrapInPromise = exports.debugCurriedP = exports.errorCurried = exports.error = exports.debugCurried = exports.debug = undefined;

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _debug = require('debug');

var _debug2 = _interopRequireDefault(_debug);

var _package = require('../package.json');

var _package2 = _interopRequireDefault(_package);

var _reporter = require('./reporter');

var _reporter2 = _interopRequireDefault(_reporter);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

function _toConsumableArray(arr) { if (Array.isArray(arr)) { for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) { arr2[i] = arr[i]; } return arr2; } else { return Array.from(arr); } }

var debug = exports.debug = _ramda2['default'].curryN(2, function (module, msg) {
  return (0, _debug2['default'])(_package2['default'].name + ':' + module)(msg);
});

var debugCurried = exports.debugCurried = _ramda2['default'].curryN(3, function (module, msg, args) {
  debug(module, msg);
  return args;
});

var error = exports.error = _ramda2['default'].ifElse(_ramda2['default'].is(Error), _ramda2['default'].compose(_reporter2['default'].error, _ramda2['default'].last, _ramda2['default'].reject(_ramda2['default'].isNil), _ramda2['default'].props(['message', 'stack'])), _ramda2['default'].compose(_reporter2['default'].error, _ramda2['default'].ifElse(_ramda2['default'].is(String), _ramda2['default'].identity, _ramda2['default'].partialRight(JSON.stringify, [null, 2]))));

var errorCurried = exports.errorCurried = _ramda2['default'].curryN(2, function (msg, args) {
  error(msg);
  return args;
});

var debugCurriedP = exports.debugCurriedP = _ramda2['default'].curryN(3, function (module, msg, args) {
  debug(module, msg);
  return Promise.resolve(args);
});

var wrapInPromise = exports.wrapInPromise = function () {
  function wrapInPromise(val) {
    return Promise.resolve(val);
  }

  return wrapInPromise;
}();

var promisify = exports.promisify = function () {
  function promisify(func) {
    return function () {
      for (var _len = arguments.length, args = Array(_len), _key = 0; _key < _len; _key++) {
        args[_key] = arguments[_key];
      }

      return new Promise(function (resolve, reject) {
        return _ramda2['default'].apply(func, [].concat(_toConsumableArray(_ramda2['default'].reject(_ramda2['default'].isNil, args)), [_ramda2['default'].ifElse(_ramda2['default'].compose(_ramda2['default'].not, _ramda2['default'].isNil, _ramda2['default'].nthArg(0)), reject, _ramda2['default'].unapply(_ramda2['default'].compose(_ramda2['default'].apply(resolve), _ramda2['default'].tail)))]));
      });
    };
  }

  return promisify;
}();