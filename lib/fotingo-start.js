'use strict';

var _commander = require('commander');

var _commander2 = _interopRequireDefault(_commander);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _util = require('./issue-tracker/util');

var _config = require('./config');

var _config2 = _interopRequireDefault(_config);

var _error = require('./error');

var _init = require('./init');

var _init2 = _interopRequireDefault(_init);

var _reporter = require('./reporter');

var _reporter2 = _interopRequireDefault(_reporter);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var _reporter$stepFactory = _reporter2['default'].stepFactory(4),
    step = _reporter$stepFactory.step,
    stepCurried = _reporter$stepFactory.stepCurried;

var getIssueId = _ramda2['default'].compose(_util.validate, _ramda2['default'].head, _ramda2['default'].prop('args'));

try {
  (function () {
    _commander2['default'].parse(process.argv);
    var issueId = getIssueId(_commander2['default']);
    step(1, 'Initializing services', 'rocket');
    (0, _init2['default'])(_config2['default'], _commander2['default']).then(function (_ref) {
      var git = _ref.git,
          issueTracker = _ref.issueTracker;
      return issueTracker.getCurrentUser().then(stepCurried(2, 'Getting issue from ' + issueTracker.name, 'bug')).then(function (user) {
        return issueTracker.getIssue(issueId).then(issueTracker.canWorkOnIssue(user)).then(stepCurried(3, 'Updating issue status', 'bookmark')).then(issueTracker.setIssueStatus({ status: issueTracker.status.IN_PROGRESS })).then(stepCurried(4, 'Creating branch to work on issue', 'tada')).then(git.createIssueBranch(_config2['default']));
      });
    }).then(_reporter2['default'].footer)['catch'](_error.handleError);
  })();
} catch (e) {
  (0, _error.handleError)(e);
  _commander2['default'].help();
}