'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _local = require('./git/local');

var _local2 = _interopRequireDefault(_local);

var _github = require('./git/github');

var _github2 = _interopRequireDefault(_github);

var _issueTracker = require('./issue-tracker');

var _issueTracker2 = _interopRequireDefault(_issueTracker);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

// config -> project -> promise
exports['default'] = _ramda2['default'].curryN(2, function (c, program) {
  return _github2['default'].init(c)().then(_local2['default'].init(c, process.cwd())).then((0, _issueTracker2['default'])(program)(c)).then(function (issueTracker) {
    return { github: _github2['default'], git: _local2['default'], issueTracker: issueTracker };
  });
});