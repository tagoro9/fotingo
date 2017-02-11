'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _readConfigFile = require('./read-config-file');

var _readConfigFile2 = _interopRequireDefault(_readConfigFile);

var _writeConfigFile = require('./write-config-file');

var _writeConfigFile2 = _interopRequireDefault(_writeConfigFile);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var config = (0, _readConfigFile2['default'])({
  git: {
    remote: 'origin',
    branch: 'master'
  },
  github: {
    base: 'master'
  },
  jira: {
    status: {},
    user: {}
  }
});

var data = {
  isGithubLoggedIn: function () {
    function isGithubLoggedIn() {
      return _ramda2['default'].not(_ramda2['default'].isNil(data.get(['github', 'token'])));
    }

    return isGithubLoggedIn;
  }(),
  isJiraLoggedIn: function () {
    function isJiraLoggedIn() {
      return _ramda2['default'].not(_ramda2['default'].or(_ramda2['default'].isNil(data.get(['jira', 'user', 'password'])), _ramda2['default'].isNil(data.get(['jira', 'user', 'login']))));
    }

    return isJiraLoggedIn;
  }(),
  update: _ramda2['default'].curryN(2, function (path, value) {
    config = _ramda2['default'].set(_ramda2['default'].lensPath(path), value)(config);
    (0, _writeConfigFile2['default'])(config);
    return value;
  }),
  get: function () {
    function get(path) {
      return _ramda2['default'].view(_ramda2['default'].lensPath(path), config);
    }

    return get;
  }()
};

exports['default'] = data;