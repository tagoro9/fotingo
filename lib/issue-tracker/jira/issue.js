'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _error = require('../../error');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

exports['default'] = function (status) {
  // issue -> boolean
  var isInNoGroup = _ramda2['default'].compose(_ramda2['default'].isEmpty, _ramda2['default'].path(['fields', 'labels']));
  // issue -> login -> Boolean
  var isInUserGroup = _ramda2['default'].converge(function (groups, labels) {
    return _ramda2['default'].any(_ramda2['default'].contains(_ramda2['default'].__, groups), labels);
  }, [_ramda2['default'].compose(_ramda2['default'].map(_ramda2['default'].prop('name')), _ramda2['default'].path(['groups', 'items']), _ramda2['default'].nthArg(1)), _ramda2['default'].compose(_ramda2['default'].map(_ramda2['default'].replace('team-', '')), _ramda2['default'].path(['fields', 'labels']), _ramda2['default'].nthArg(0))]);
  // issue -> login -> Boolean
  var isAssignedToUser = function () {
    function isAssignedToUser(issue, user) {
      return _ramda2['default'].pathEq(['fields', 'assignee', 'key'], user.key, issue);
    }

    return isAssignedToUser;
  }();
  // issue -> Boolean
  var isUnassigned = _ramda2['default'].pathSatisfies(_ramda2['default'].isNil, ['fields', 'assignee']);
  // issue -> Boolean
  var hasValidStatus = _ramda2['default'].compose(_ramda2['default'].contains(_ramda2['default'].__, [status.SELECTED_FOR_DEVELOPMENT, status.BACKLOG]), Number, _ramda2['default'].path(['fields', 'status', 'id']));

  return {
    // user -> issue -> issue
    canWorkOnIssue: _ramda2['default'].curryN(2, _ramda2['default'].flip(_ramda2['default'].ifElse(_ramda2['default'].either(isAssignedToUser, _ramda2['default'].both(_ramda2['default'].both(hasValidStatus, isUnassigned), _ramda2['default'].either(isInUserGroup, isInNoGroup))), _ramda2['default'].nthArg(0, Promise.resolve), (0, _error.throwControlledError)(_error.errors.jira.cantWorkOnIssue))))
  };
};