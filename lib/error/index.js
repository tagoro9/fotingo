'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.errors = exports.catchPromiseAndThrow = exports.handleError = exports.throwControlledError = exports.ControlledError = undefined;

var _slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _errors = require('./errors');

var _errors2 = _interopRequireDefault(_errors);

var _util = require('../util');

var _reporter = require('../reporter');

var _reporter2 = _interopRequireDefault(_reporter);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

function _classCallCheck(instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } }

function _possibleConstructorReturn(self, call) { if (!self) { throw new ReferenceError("this hasn't been initialised - super() hasn't been called"); } return call && (typeof call === "object" || typeof call === "function") ? call : self; }

function _inherits(subClass, superClass) { if (typeof superClass !== "function" && superClass !== null) { throw new TypeError("Super expression must either be null or a function, not " + typeof superClass); } subClass.prototype = Object.create(superClass && superClass.prototype, { constructor: { value: subClass, enumerable: false, writable: true, configurable: true } }); if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass; }

function _extendableBuiltin(cls) {
  function ExtendableBuiltin() {
    var instance = Reflect.construct(cls, Array.from(arguments));
    Object.setPrototypeOf(instance, Object.getPrototypeOf(this));
    return instance;
  }

  ExtendableBuiltin.prototype = Object.create(cls.prototype, {
    constructor: {
      value: cls,
      enumerable: false,
      writable: true,
      configurable: true
    }
  });

  if (Object.setPrototypeOf) {
    Object.setPrototypeOf(ExtendableBuiltin, cls);
  } else {
    ExtendableBuiltin.__proto__ = cls;
  }

  return ExtendableBuiltin;
}

var ControlledError = exports.ControlledError = function (_extendableBuiltin2) {
  _inherits(ControlledError, _extendableBuiltin2);

  function ControlledError(message) {
    var parameters = arguments.length > 1 && arguments[1] !== undefined ? arguments[1] : {};

    _classCallCheck(this, ControlledError);

    return _possibleConstructorReturn(this, (ControlledError.__proto__ || Object.getPrototypeOf(ControlledError)).call(this, _ramda2['default'].compose(_ramda2['default'].reduce(function (msg, _ref) {
      var _ref2 = _slicedToArray(_ref, 2),
          k = _ref2[0],
          v = _ref2[1];

      return _ramda2['default'].replace('${' + k + '}', v, msg);
    }, message), _ramda2['default'].toPairs)(parameters)));
  }

  return ControlledError;
}(_extendableBuiltin(Error));

var throwControlledError = exports.throwControlledError = function () {
  function throwControlledError(message, parameters) {
    return function () {
      throw new ControlledError(message, parameters);
    };
  }

  return throwControlledError;
}();

// Error -> Boolean
var isKnownError = _ramda2['default'].either(_ramda2['default'].is(ControlledError), _ramda2['default'].propEq('message', 'canceled'));
var exit = function () {
  function exit(code) {
    return function () {
      return process.exit(code);
    };
  }

  return exit;
}();
var userIsExiting = _ramda2['default'].compose(_ramda2['default'].equals('canceled'), _ramda2['default'].prop('message'));
var handleErrorAndExit = _ramda2['default'].compose(exit(0), _util.error, _ramda2['default'].prop('message'));
var handleUnknownError = _ramda2['default'].compose(exit(1), _util.error);
var sayBye = function () {
  function sayBye() {
    return _reporter2['default'].log('Hasta la vista baby!', 'wave');
  }

  return sayBye;
}();
var handleError = exports.handleError = _ramda2['default'].ifElse(isKnownError, _ramda2['default'].ifElse(userIsExiting, sayBye, handleErrorAndExit), handleUnknownError);
// String -> Promise -> Promise
var catchPromiseAndThrow = exports.catchPromiseAndThrow = function () {
  function catchPromiseAndThrow(module, e) {
    return function (p) {
      return p['catch'](function (err) {
        if (_ramda2['default'].is(Function, e)) {
          throwControlledError(e(err))(err);
        } else {
          _ramda2['default'].compose(throwControlledError(e), (0, _util.debugCurried)(module, err))(err);
        }
      });
    };
  }

  return catchPromiseAndThrow;
}();
exports.errors = _errors2['default'];