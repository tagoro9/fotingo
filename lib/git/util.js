'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.getIssueIdFromBranch = exports.getProject = exports.createBranchName = exports.ISSUE_TYPES = undefined;

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var ISSUE_TYPES = exports.ISSUE_TYPES = {
  Bug: 'b',
  Feature: 'f',
  Story: 'f',
  'Sub-task': 'f',
  Task: 'c'
};

var sanitizeSummary = _ramda2['default'].compose(_ramda2['default'].take(72), _ramda2['default'].replace(/(_|-)$/, ''), _ramda2['default'].replace(/--+/, '-'), _ramda2['default'].replace(/__+/, '_'), _ramda2['default'].replace(/\s|\(|\)/g, '_'), _ramda2['default'].replace(/\/|\./g, '-'), _ramda2['default'].replace(/,|\[|]|"|'|”|“|@|’|`|:|\$|\?|\*/g, ''), _ramda2['default'].toLower);

// This should go in the issue?
var createBranchName = exports.createBranchName = function () {
  function createBranchName(_ref) {
    var key = _ref.key,
        _ref$fields = _ref.fields,
        type = _ref$fields.issuetype.name,
        summary = _ref$fields.summary;
    return ISSUE_TYPES[type] + '/' + key.toLowerCase() + '_' + sanitizeSummary(summary);
  }

  return createBranchName;
}();

var getProject = exports.getProject = _ramda2['default'].compose(_ramda2['default'].nth(1), _ramda2['default'].match(/\/((\w|-)+)$/));

var getIssueIdFromBranch = exports.getIssueIdFromBranch = _ramda2['default'].compose(_ramda2['default'].ifElse(_ramda2['default'].isNil, _ramda2['default'].identity, _ramda2['default'].toUpper), _ramda2['default'].last, _ramda2['default'].match(/\w\/(\w+-\d+)/));