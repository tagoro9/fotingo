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

var status = exports.status = {
  BACKLOG: 'BACKLOG',
  IN_PROGRESS: 'IN_PROGRESS',
  IN_REVIEW: 'IN_PREVIEW',
  SELECTED_FOR_DEVELOPMENT: 'SELECTED_FOR_DEVELOPMENT'
};

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
    question: 'What are the ids for the transitions that represent\n' + 'Backlog, to do, in progress, in review (enter a comma separated list)?'
  }), _reporter2['default'].info, _ramda2['default'].concat('These are the possible transitions found for the issue:\n')));

  return _ramda2['default'].ifElse(_ramda2['default'].compose(_ramda2['default'].either(_ramda2['default'].isNil, _ramda2['default'].isEmpty), _ramda2['default'].invoker(1, 'get')(['jira', 'status'])), _ramda2['default'].partial(_ramda2['default'].compose(askForStatus, _ramda2['default'].join('\n'), _ramda2['default'].map(function (_ref3) {
    var name = _ref3.name,
        id = _ref3.id;
    return ' * ' + name + ': ' + id;
  }), _ramda2['default'].map(_ramda2['default'].prop('to')), _ramda2['default'].prop('transitions')), [issue]), _ramda2['default'].compose(_util.wrapInPromise, _ramda2['default'].invoker(1, 'get')(['jira', 'status'])))(config);
});