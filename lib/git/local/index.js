'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var _nodegit = require('nodegit');

var _nodegit2 = _interopRequireDefault(_nodegit);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _error = require('../../error');

var _util = require('../util');

var _util2 = require('../../util');

var _package = require('../../../package.json');

var _package2 = _interopRequireDefault(_package);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var fetchOptions = {
  callbacks: {
    certificateCheck: _ramda2['default'].always(1),
    credentials: _ramda2['default'].compose(_nodegit2['default'].Cred.sshKeyFromAgent, (0, _util2.debugCurried)('git', 'Getting authentication from SSH agent'),
    // TODO Detect ssh key not present
    _ramda2['default'].nthArg(1))
  }
};

var repository = null;

var getCurrentBranchName = _ramda2['default'].composeP(function (ref) {
  return _nodegit2['default'].Branch.name(ref);
}, function () {
  return repository.head();
});

var footerRegex = /^(closes|fixes)\s+((#\w+-\d+)(,?\s+#\w+-\d+)*)\s*$/i;

// String -> Array
var getIssues = _ramda2['default'].compose(_ramda2['default'].reject(_ramda2['default'].isEmpty), _ramda2['default'].map(_ramda2['default'].trim), _ramda2['default'].split(','), _ramda2['default'].when(_ramda2['default'].isNil, _ramda2['default'].always('')), _ramda2['default'].nth(2), _ramda2['default'].match(footerRegex), _ramda2['default'].last, _ramda2['default'].reject(_ramda2['default'].isEmpty), _ramda2['default'].split('\n'));

// String -> String
var formatMessage = _ramda2['default'].compose(_ramda2['default'].join('\n'), _ramda2['default'].when(_ramda2['default'].compose(_ramda2['default'].lt(1), _ramda2['default'].length), _ramda2['default'].init), _ramda2['default'].reject(_ramda2['default'].isEmpty), _ramda2['default'].split('\n'));

// Commit -> Object
var transformCommit = _ramda2['default'].compose(_ramda2['default'].converge(_ramda2['default'].unapply(function (_ref) {
  var _ref2 = _slicedToArray(_ref, 2),
      issues = _ref2[0],
      message = _ref2[1];

  return { issues: issues, message: message };
}), [getIssues, formatMessage]), _ramda2['default'].invoker(0, 'message'));

exports['default'] = {
  init: function () {
    function init(config, pathToRepo) {
      return function () {
        (0, _util2.debug)('git', 'Initializing ' + pathToRepo + ' repository');
        return _nodegit2['default'].Repository.open(pathToRepo).then(function (repo) {
          repository = repo;
          return Promise.resolve(undefined);
        })['catch']((0, _error.throwControlledError)(_error.errors.git.couldNotInitializeRepo, { pathToRepo: pathToRepo }));
      };
    }

    return init;
  }(),
  createIssueBranch: _ramda2['default'].curryN(2, function (config, issue) {
    (0, _util2.debug)('git', 'Creating branch for issue');
    var name = (0, _util.createBranchName)(issue);

    var _config$get = config.get(['git']),
        remote = _config$get.remote,
        branch = _config$get.branch;

    (0, _util2.debug)('git', 'Fetching data from remote');
    // We should fetch -> co master -> reset to origin/master -> create branch
    return repository.fetch(remote, fetchOptions).then((0, _util2.debugCurriedP)('git', 'Getting local repository status')).then(function () {
      return repository.getStatus();
    }).then(_ramda2['default'].ifElse(_ramda2['default'].isEmpty, _ramda2['default'].identity, function () {
      return _nodegit2['default'].Stash.save((0, _util2.debugCurried)('git', 'Stashing changes', repository), repository.defaultSignature(), 'auto generated stash by ' + _package2['default'].name, _nodegit2['default'].Stash.FLAGS.INCLUDE_UNTRACKED);
    })).then(function () {
      return repository.getBranchCommit(remote + '/' + branch);
    }).then((0, _util2.debugCurriedP)('git', 'Creating new branch')).then(function (commit) {
      return repository.createBranch(name, commit);
    }).then(function () {
      return repository.checkoutBranch(name);
    });
  }),
  pushBranchToGithub: _ramda2['default'].curryN(1, function (config) {
    // TODO implmement this
    return Promise.resolve(config);
  }),
  extractIssueFromCurrentBranch: function () {
    function extractIssueFromCurrentBranch() {
      return _ramda2['default'].composeP((0, _util2.debugCurriedP)('git', 'Extracting issue from current branch'), _ramda2['default'].compose(_util2.wrapInPromise, _util.getIssueIdFromBranch), getCurrentBranchName)();
    }

    return extractIssueFromCurrentBranch;
  }(),
  getBranchInfo: function () {
    function getBranchInfo() {
      (0, _util2.debug)('git', 'Getting branch commit history');
      return Promise.all([repository.getHeadCommit(), repository.getBranchCommit('origin/master')]).then(_ramda2['default'].when(_ramda2['default'].compose(_ramda2['default'].not, _ramda2['default'].allUniq, _ramda2['default'].map(_ramda2['default'].compose(_ramda2['default'].toString, _ramda2['default'].invoker(0, 'id')))), (0, _error.throwControlledError)(_error.errors.git.noChanges))).then(function (_ref3) {
        var _ref4 = _slicedToArray(_ref3, 2),
            latestCommit = _ref4[0],
            latestMasterCommit = _ref4[1];

        return Promise.all([_nodegit2['default'].Merge.base(repository, latestCommit, latestMasterCommit).then(function (latestCommonCommit) {
          (0, _util2.debug)('git', 'Created history walker. Latest common commit: ' + latestCommonCommit);
          var historyWalker = repository.createRevWalk();
          var commitStopper = function () {
            function commitStopper(commit) {
              return !latestCommonCommit.equal(commit.id());
            }

            return commitStopper;
          }();
          historyWalker.push(latestCommit);
          return historyWalker.getCommitsUntil(commitStopper);
        }).then(function (commits) {
          return _ramda2['default'].compose(_ramda2['default'].reverse, _ramda2['default'].map(transformCommit), _ramda2['default'].init)(commits);
        }), getCurrentBranchName()]).then(function (_ref5) {
          var _ref6 = _slicedToArray(_ref5, 2),
              commits = _ref6[0],
              name = _ref6[1];

          return { name: name, commits: commits };
        });
      });
    }

    return getBranchInfo;
  }()
};