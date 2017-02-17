'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _child_process = require('child_process');

var _child_process2 = _interopRequireDefault(_child_process);

var _fs = require('fs');

var _fs2 = _interopRequireDefault(_fs);

var _github = require('github');

var _github2 = _interopRequireDefault(_github);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _util = require('../util');

var _util2 = require('../../util');

var _error = require('../../error');

var _reporter = require('../../reporter');

var _reporter2 = _interopRequireDefault(_reporter);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var github = new _github2['default']({});

var getCurrentUser = _ramda2['default'].compose(_ramda2['default'].partial(_ramda2['default'].__, [{}]), _util2.promisify)(github.users.get);

var createPullRequest = (0, _util2.promisify)(github.pullRequests.create);

var authenticate = _ramda2['default'].compose(_util2.wrapInPromise, function (token) {
  return github.authenticate(token);
}, _ramda2['default'].set(_ramda2['default'].lensProp('token'), _ramda2['default'].__, { type: 'token' }));

var readUserToken = _ramda2['default'].compose(_ramda2['default'].partial(_reporter2['default'].question, [{ question: 'Introduce a Github personal token' }]), (0, _util2.debugCurried)('github', 'Github account not set'));

var authenticateAndGetCurrentUser = _ramda2['default'].composeP(_ramda2['default'].compose((0, _error.catchPromiseAndThrow)('github', function (e) {
  switch (e.code) {
    case '500':
      return _error.errors.github.cantConnect;
    default:
      return _error.errors.github.tokenInvalid;
  }
}), getCurrentUser), authenticate);

// Object -> Object -> String
var buildPullRequestTitle = _ramda2['default'].ifElse(_ramda2['default'].isNil, _ramda2['default'].compose(_ramda2['default'].concat(_ramda2['default'].__, '\n'), _ramda2['default'].prop('name'), _ramda2['default'].nthArg(1)), function (issue) {
  return _util.ISSUE_TYPES[issue.fields.issuetype.name] + '/' + issue.key + ' ' + _ramda2['default'].take(60, issue.fields.summary) + '\n';
});

// Array -> String
var buildPullRequestBody = _ramda2['default'].compose(_ramda2['default'].join('\n'), _ramda2['default'].map(_ramda2['default'].converge(_ramda2['default'].concat, [_ramda2['default'].compose(_ramda2['default'].concat('* '), _ramda2['default'].prop('header')), _ramda2['default'].compose(_ramda2['default'].ifElse(_ramda2['default'].isNil, _ramda2['default'].always(''), _ramda2['default'].replace(/\n|^/g, '\n  ')), _ramda2['default'].prop('body'))])));
// Array -> String
var buildPullRequestFooter = function () {
  function buildPullRequestFooter(issueRoot) {
    return _ramda2['default'].compose(_ramda2['default'].ifElse(_ramda2['default'].isEmpty, _ramda2['default'].always(''), _ramda2['default'].compose(_ramda2['default'].concat('\nFixes '), _ramda2['default'].join(', '))), _ramda2['default'].map(function (_ref) {
      var raw = _ref.raw,
          issue = _ref.issue;
      return '[' + raw + '](' + issueRoot + issue + ')';
    }), _ramda2['default'].uniqBy(_ramda2['default'].prop('issue')));
  }

  return buildPullRequestFooter;
}();

// Object -> Object -> String
var buildPullRequestDescription = function () {
  function buildPullRequestDescription(issueRoot) {
    return _ramda2['default'].converge(_ramda2['default'].compose(_util2.wrapInPromise, _ramda2['default'].unapply(_ramda2['default'].join('\n'))), [buildPullRequestTitle, _ramda2['default'].compose(buildPullRequestBody, _ramda2['default'].prop('commits'), _ramda2['default'].nthArg(1)), _ramda2['default'].compose(buildPullRequestFooter(issueRoot), _ramda2['default'].prop('issues'), _ramda2['default'].nthArg(1))]);
  }

  return buildPullRequestDescription;
}();

// String -> String -> *
var writeFile = _ramda2['default'].curryN(2, function (file, content) {
  return (0, _util2.promisify)(_fs2['default'].writeFile)(file, content, 'utf8');
});
// String -> * -> String
var readFile = function () {
  function readFile(file) {
    return function () {
      return (0, _util2.promisify)(_fs2['default'].readFile)(file, 'utf8');
    };
  }

  return readFile;
}();
// String -> String -> String
var deleteFile = function () {
  function deleteFile(file) {
    return function (content) {
      return (0, _util2.promisify)(_fs2['default'].unlink)(file).then(function () {
        return (0, _util2.wrapInPromise)(content);
      });
    };
  }

  return deleteFile;
}();
// * -> String
var editFile = function () {
  function editFile(file) {
    return function () {
      return new Promise(function (resolve, reject) {
        var vim = _child_process2['default'].spawn('vim', [file], { stdio: 'inherit' });
        vim.on('exit', _ramda2['default'].ifElse(_ramda2['default'].equals(0), resolve, reject));
      });
    };
  }

  return editFile;
}();

// Object -> String -> String
var allowUserToEditPullRequest = function () {
  function allowUserToEditPullRequest(description) {
    var prFile = '/tmp/fotingo-pr-' + Date.now();
    return _ramda2['default'].composeP(_ramda2['default'].compose(_util2.wrapInPromise, _ramda2['default'].trim), deleteFile(prFile), readFile(prFile), editFile(prFile), writeFile(prFile))(description);
  }

  return allowUserToEditPullRequest;
}();

var submitPullRequest = _ramda2['default'].curryN(4, function (config, project, branchInfo, description) {
  return createPullRequest({
    owner: config.get(['github', 'owner']),
    repo: project,
    head: branchInfo.name,
    base: config.get(['github', 'base']),
    title: _ramda2['default'].compose(_ramda2['default'].head, _ramda2['default'].split('\n'))(description),
    body: _ramda2['default'].compose(_ramda2['default'].join('\n'), _ramda2['default'].tail, _ramda2['default'].split('\n'))(description)
  });
});

exports['default'] = {
  init: function () {
    function init(config) {
      return function () {
        (0, _util2.debug)('github', 'Initializing Github api');

        var doLogin = _ramda2['default'].composeP(authenticateAndGetCurrentUser, config.update(['github', 'token']), readUserToken);

        var configPromise = _ramda2['default'].isNil(config.get(['github', 'owner'])) ? _ramda2['default'].composeP(config.update(['github', 'owner']), _reporter2['default'].question)({ question: 'What\'s the github repository owner?' }) : (0, _util2.wrapInPromise)(config.get(['github', 'owner']));

        return configPromise.then(function () {
          if (config.isGithubLoggedIn()) {
            (0, _util2.debug)('github', 'User token is present. Using current authentication');
            return authenticateAndGetCurrentUser(config.get(['github', 'token']))
            // TODO differentiate error codes so only login is attempted when tokenInvalid
            ['catch'](_ramda2['default'].composeP(doLogin, (0, _util2.debugCurriedP)('github', 'Current authentication failed. Attempting login')));
          }
          (0, _util2.debug)('github', 'No user token present. Attempting login');
          return doLogin();
        });
      };
    }

    return init;
  }(),
  // Object -> Array -> Promise
  createPullRequest: _ramda2['default'].curryN(5, function (config, project, issue, issueRoot, branchInfo) {
    return _ramda2['default'].composeP(_ramda2['default'].ifElse(_ramda2['default'].isEmpty, (0, _error.throwControlledError)(_error.errors.github.pullRequestDescriptionInvalid),
    // Assign the PR link to all the issues that were created
    _ramda2['default'].composeP(_ramda2['default'].compose(_util2.wrapInPromise, _ramda2['default'].set(_ramda2['default'].lensProp('pullRequest'), _ramda2['default'].__, { branchInfo: branchInfo })), submitPullRequest(config, project, branchInfo))), (0, _util2.debugCurriedP)('github', 'Submitting pull request to github'), allowUserToEditPullRequest, (0, _util2.debugCurriedP)('github', 'Editing pull request description'), buildPullRequestDescription(issueRoot))(issue, branchInfo);
  })
};