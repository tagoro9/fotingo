'use strict';

var _commander = require('commander');

var _commander2 = _interopRequireDefault(_commander);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _util = require('./git/util');

var _config = require('./config');

var _config2 = _interopRequireDefault(_config);

var _error = require('./error');

var _init = require('./init');

var _init2 = _interopRequireDefault(_init);

var _reporter = require('./reporter');

var _reporter2 = _interopRequireDefault(_reporter);

var _util2 = require('./util');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

try {
  (function () {
    _commander2['default'].option('-n, --no-branch-issue', 'Do not pick issue from branch name').option('-s, --simple', 'Do not use any issue tracker').parse(process.argv);

    var shouldGetIssue = _ramda2['default'].partial(_ramda2['default'].both(_ramda2['default'].compose(_ramda2['default'].equals(true), _ramda2['default'].prop('branchIssue')), _ramda2['default'].compose(_ramda2['default'].not, _ramda2['default'].equals(true), _ramda2['default'].prop('simple'))), [_commander2['default']]);
    var getTotalSteps = _ramda2['default'].ifElse(_ramda2['default'].propEq('simple', true), _ramda2['default'].always(4), _ramda2['default'].ifElse(_ramda2['default'].propEq('branchIssue', false), _ramda2['default'].always(5), _ramda2['default'].always(7)));

    var _reporter$stepFactory = _reporter2['default'].stepFactory(getTotalSteps(_commander2['default'])),
        step = _reporter$stepFactory.step,
        stepCurried = _reporter$stepFactory.stepCurried;

    var stepOffset = shouldGetIssue() ? 0 : 2;
    var project = (0, _util.getProject)(process.cwd());
    step(1, 'Initializing services', 'rocket');
    (0, _init2['default'])(_config2['default'], _commander2['default']).then(function (_ref) {
      var git = _ref.git,
          github = _ref.github,
          issueTracker = _ref.issueTracker;
      return (0, _util2.wrapInPromise)(step(2, 'Pushing current branch to Github', 'arrow_up')).then(_ramda2['default'].partial(git.pushBranchToGithub, [_config2['default']])).then(_ramda2['default'].ifElse(shouldGetIssue, _ramda2['default'].compose(issueTracker.getIssue, stepCurried(4, 'Getting issue from ' + issueTracker.name, 'bug'), git.extractIssueFromCurrentBranch, stepCurried(3, 'Extracting issue from current branch', 'mag_right')), _ramda2['default'].always(undefined))).then(function (issue) {
        step(5 - stepOffset, 'Getting your commit history', 'books');
        return git.getBranchInfo(issue).then(step(6 - stepOffset, 'Creating pull request', 'speaker')).then(github.createPullRequest(_config2['default'], project, issue, issueTracker.issueRoot));
      }).then(_ramda2['default'].ifElse(_ramda2['default'].partial(_ramda2['default'].compose(_ramda2['default'].not, _ramda2['default'].propEq('simple', true)), [_commander2['default']]), function (_ref2) {
        var issues = _ref2.branchInfo.issues,
            pullRequest = _ref2.pullRequest;
        return _ramda2['default'].compose(function (promise) {
          return promise.then(_ramda2['default'].always((0, _util2.wrapInPromise)({ pullRequest: pullRequest })));
        }, function (promises) {
          return Promise.all(promises);
        }, _ramda2['default'].map(_ramda2['default'].composeP(_ramda2['default'].partial(issueTracker.setIssueStatus, [{ status: issueTracker.status.IN_REVIEW, comment: 'PR: ' + pullRequest.html_url }]), issueTracker.getIssue, _ramda2['default'].compose(_util2.wrapInPromise, _ramda2['default'].prop('issue')))), stepCurried(7 - stepOffset, 'Setting issues to code review on ' + issueTracker.name, 'bookmark'))(issues);
      }, _ramda2['default'].identity));
    }).then(_ramda2['default'].compose(_reporter2['default'].footer, _ramda2['default'].ifElse(_ramda2['default'].isNil, _ramda2['default'].identity, _ramda2['default'].path(['pullRequest', 'html_url']))))['catch'](_error.handleError);
  })();
} catch (e) {
  (0, _error.handleError)(e);
  _commander2['default'].help();
}