'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _extends = Object.assign || function (target) { for (var i = 1; i < arguments.length; i++) { var source = arguments[i]; for (var key in source) { if (Object.prototype.hasOwnProperty.call(source, key)) { target[key] = source[key]; } } } return target; };

exports['default'] = function (rootUrl) {
  var makeUrl = _ramda2['default'].concat(rootUrl);
  var auth = {};
  var jar = _request2['default'].jar();
  var serverCall = _ramda2['default'].curry(function (method, url, _ref) {
    var form = _ref.form,
        body = _ref.body;
    return new Promise(function (resolve, reject) {
      (0, _util.debug)('http', 'Performing ' + method + ' ' + makeUrl(url) + ' ' + (body ? 'with body ' + JSON.stringify(body, null, 2) : ''));
      return (0, _request2['default'])({ url: makeUrl(url), body: body, form: form, headers: headers, jar: jar, json: true, method: method, auth: auth }, handleServerResponse(resolve, reject));
    });
  });
  var setCookieToJar = _ramda2['default'].compose(_ramda2['default'].bind(_ramda2['default'].partialRight(jar.setCookie, [rootUrl]), jar), _request2['default'].cookie);

  return {
    post: serverCall('POST'),
    get: serverCall('GET', _ramda2['default'].__, {}),
    setAuth: function () {
      function setAuth(_ref2) {
        var login = _ref2.login,
            password = _ref2.password;
        auth = _extends({}, auth, { user: login, pass: password });
      }

      return setAuth;
    }(),
    setCookie: _ramda2['default'].ifElse(_ramda2['default'].is(Array), _ramda2['default'].map(setCookieToJar), setCookieToJar)
  };
};

var _request = require('request');

var _request2 = _interopRequireDefault(_request);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _util = require('../util');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var handleServerResponse = function () {
  function handleServerResponse(resolve, reject) {
    return _ramda2['default'].ifElse(_ramda2['default'].compose(_ramda2['default'].not, _ramda2['default'].isNil, _ramda2['default'].nthArg(0)), reject, _ramda2['default'].ifElse(_ramda2['default'].compose(_ramda2['default'].propSatisfies(_ramda2['default'].gt(400), 'statusCode'), _ramda2['default'].nthArg(1)), _ramda2['default'].unapply(_ramda2['default'].compose(resolve, function (args) {
      return { response: args[1], body: args[2] };
    })), _ramda2['default'].compose(reject, _ramda2['default'].unapply(function (args) {
      _ramda2['default'].compose((0, _util.debug)('http'), _ramda2['default'].concat('Request failed with status code '), _ramda2['default'].prop('statusCode'), _ramda2['default'].nth(1))(args);
      return args[2];
    }))));
  }

  return handleServerResponse;
}();

var headers = { accept: 'application/json' };