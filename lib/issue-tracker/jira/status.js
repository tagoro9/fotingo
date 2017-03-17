'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.status = undefined;

var _slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _util = require('../../util');

var _reporter = require('../../reporter');

var _reporter2 = _interopRequireDefault(_reporter);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

function _defineProperty(obj, key, value) { if (key in obj) { Object.defineProperty(obj, key, { value: value, enumerable: true, configurable: true, writable: true }); } else { obj[key] = value; } return obj; }

var status = exports.status = {
  BACKLOG: 'BACKLOG',
  DONE: 'DONE',
  IN_PROGRESS: 'IN_PROGRESS',
  IN_REVIEW: 'IN_REVIEW',
  SELECTED_FOR_DEVELOPMENT: 'SELECTED_FOR_DEVELOPMENT'
};

var statusRegex = {
  BACKLOG: /backlog/i,
  IN_PROGRESS: /in progress/i,
  IN_REVIEW: /review/i,
  DONE: /done/i,
  SELECTED_FOR_DEVELOPMENT: /(todo)|(to do)|(selected for development)/i
};

var statusMatcher = function () {
  function statusMatcher(statusToFind) {
    return _ramda2['default'].compose(_ramda2['default'].ifElse(_ramda2['default'].isNil, _ramda2['default'].identity, _ramda2['default'].compose(parseInt, _ramda2['default'].prop('id'))), _ramda2['default'].find(_ramda2['default'].compose(_ramda2['default'].test(statusRegex[statusToFind]), _ramda2['default'].prop('name'))));
  }

  return statusMatcher;
}();

exports['default'] = _ramda2['default'].curryN(2, function (config, issue) {

  var askForStatus = _ramda2['default'].composeP(config.update(['jira', 'status']), _ramda2['default'].compose(_util.wrapInPromise, function (_ref) {
    var _ref2 = _slicedToArray(_ref, 4),
        BACKLOG = _ref2[0],
        SELECTED_FOR_DEVELOPMENT = _ref2[1],
        IN_PROGRESS = _ref2[2],
        IN_REVIEW = _ref2[3];

    return {
      BACKLOG: BACKLOG, SELECTED_FOR_DEVELOPMENT: SELECTED_FOR_DEVELOPMENT, IN_PROGRESS: IN_PROGRESS, IN_REVIEW: IN_REVIEW
    };
  }), _ramda2['default'].compose(_util.wrapInPromise, JSON.parse), _ramda2['default'].compose(_util.wrapInPromise, _ramda2['default'].concat('['), _ramda2['default'].concat(_ramda2['default'].__, ']')), _ramda2['default'].compose(_reporter2['default'].question, _ramda2['default'].always({
    question: 'Please, insert a comma separated list of the ids'
  }), _reporter2['default'].info, _ramda2['default'].concat(_ramda2['default'].__, '\n     We need you to input the ids of the \'Backlog\', \'To do\', \'In progress\', \'In review\'\n    ' + ' (one state can represent multiple values (e.g. \'Backlog\' and \'To do\' could be the same id).\n     ' + 'It all depends on your workflow.\n'), _ramda2['default'].concat('In order to update the jira issues correctly, we need to know a little bit more about your workflow.\n     ' + 'We need to identify the different states an issue can be on. We tried inferring those values, but we\n     ' + 'were unable to do so. We are looking for 4 specific states: \'Backlog\', \'To do\', \'In progress\'\n     ' + 'and \'In review\'. These are the names and ids of the states the issue can transition to:\n     ')));

  var inferStatus = _ramda2['default'].compose(_ramda2['default'].ifElse(_ramda2['default'].compose(_ramda2['default'].any(_ramda2['default'].isNil), _ramda2['default'].values), _ramda2['default'].compose(askForStatus, _ramda2['default'].concat(_ramda2['default'].__, '\n'), _ramda2['default'].concat('\n'), _ramda2['default'].join('\n'), _ramda2['default'].map(function (_ref3) {
    var name = _ref3.name,
        id = _ref3.id;
    return '         * ' + name + ': ' + id;
  }), _ramda2['default'].prop('transitions')), _ramda2['default'].compose(_util.wrapInPromise, config.update(['jira', 'status']), _ramda2['default'].omit(['transitions']))), function (transitions) {
    var _ref4;

    return _ref4 = {}, _defineProperty(_ref4, status.BACKLOG, statusMatcher(status.BACKLOG)(transitions) || statusMatcher(status.SELECTED_FOR_DEVELOPMENT)(transitions)), _defineProperty(_ref4, status.IN_PROGRESS, statusMatcher(status.IN_PROGRESS)(transitions) || statusMatcher(status.DONE)(transitions)), _defineProperty(_ref4, status.IN_REVIEW, statusMatcher(status.IN_REVIEW)(transitions)), _defineProperty(_ref4, status.SELECTED_FOR_DEVELOPMENT, statusMatcher(status.SELECTED_FOR_DEVELOPMENT)(transitions)), _defineProperty(_ref4, 'transitions', transitions), _ref4;
  });

  return _ramda2['default'].ifElse(_ramda2['default'].compose(_ramda2['default'].either(_ramda2['default'].isNil, _ramda2['default'].isEmpty), _ramda2['default'].invoker(1, 'get')(['jira', 'status'])), _ramda2['default'].partial(_ramda2['default'].compose(inferStatus, _ramda2['default'].map(_ramda2['default'].prop('to')), _ramda2['default'].prop('transitions')), [issue]), _ramda2['default'].compose(_util.wrapInPromise, _ramda2['default'].invoker(1, 'get')(['jira', 'status'])))(config);
});