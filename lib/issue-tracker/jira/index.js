'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _slicedToArray = function () { function sliceIterator(arr, i) { var _arr = []; var _n = true; var _d = false; var _e = undefined; try { for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) { _arr.push(_s.value); if (i && _arr.length === i) break; } } catch (err) { _d = true; _e = err; } finally { try { if (!_n && _i["return"]) _i["return"](); } finally { if (_d) throw _e; } } return _arr; } return function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { return sliceIterator(arr, i); } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } }; }();

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _httpClient2 = require('../http-client');

var _httpClient3 = _interopRequireDefault(_httpClient2);

var _util = require('../../util');

var _reporter = require('../../reporter');

var _reporter2 = _interopRequireDefault(_reporter);

var _error = require('../../error');

var _issue = require('./issue');

var _issue2 = _interopRequireDefault(_issue);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

exports['default'] = function (config) {
  return function () {
    (0, _util.debug)('jira', 'Initializing Jira api');
    var root = config.get(['jira', 'root']) ? (0, _util.wrapInPromise)(config.get(['jira', 'root'])) : _ramda2['default'].composeP(config.update(['jira', 'root']), _reporter2['default'].question)({ question: 'What\'s your jira root?' });
    var statusPromise = _ramda2['default'].either(_ramda2['default'].isNil, _ramda2['default'].isEmpty)(config.get(['jira', 'status'])) ? _ramda2['default'].composeP(config.update(['jira', 'status']), _ramda2['default'].compose(_util.wrapInPromise, function (_ref) {
      var _ref2 = _slicedToArray(_ref, 4),
          BACKLOG = _ref2[0],
          SELECTED_FOR_DEVELOPMENT = _ref2[1],
          IN_PROGRESS = _ref2[2],
          IN_REVIEW = _ref2[3];

      return {
        BACKLOG: BACKLOG, SELECTED_FOR_DEVELOPMENT: SELECTED_FOR_DEVELOPMENT, IN_PROGRESS: IN_PROGRESS, IN_REVIEW: IN_REVIEW
      };
    }), _ramda2['default'].compose(_util.wrapInPromise, JSON.parse), _ramda2['default'].compose(_util.wrapInPromise, _ramda2['default'].concat('['), _ramda2['default'].concat(_ramda2['default'].__, ']')), _reporter2['default'].question)({ question: 'What are your jira step ids (Backlog, to do, in progress, in review)? Enter a comma separated list' }) : (0, _util.wrapInPromise)(config.get(['jira', 'status']));

    return Promise.all([root, statusPromise]).then(function (_ref3) {
      var _ref4 = _slicedToArray(_ref3, 2),
          jiraRoot = _ref4[0],
          status = _ref4[1];

      var _httpClient = (0, _httpClient3['default'])(jiraRoot),
          get = _httpClient.get,
          post = _httpClient.post,
          setAuth = _httpClient.setAuth;

      var issueRoot = jiraRoot + '/browse/';
      var readUserInfo = function () {
        function readUserInfo() {
          // It doesn't feel like logger should be used here
          (0, _util.debug)('jira', 'Reading user login info');
          var readUsernamePromise = _reporter2['default'].question({ question: 'What\'s your Jira username?' });
          var readPasswordPromise = readUsernamePromise.then(_ramda2['default'].partial(_reporter2['default'].question, [{ question: 'What\'s your Jira password?', password: true }]));
          return Promise.all([readUsernamePromise, readPasswordPromise]).then(function (_ref5) {
            var _ref6 = _slicedToArray(_ref5, 2),
                login = _ref6[0],
                password = _ref6[1];

            return { login: login, password: password };
          });
        }

        return readUserInfo;
      }();

      var getCurrentUser = function () {
        function getCurrentUser() {
          return get('/rest/api/2/myself?expand=groups').then(_ramda2['default'].prop('body'));
        }

        return getCurrentUser;
      }();

      var doLogin = _ramda2['default'].composeP(getCurrentUser, setAuth, config.update(['jira', 'user']), readUserInfo);
      var loginPromise = void 0;
      if (config.isJiraLoggedIn()) {
        setAuth(config.get(['jira', 'user']));
        loginPromise = getCurrentUser()['catch'](_ramda2['default'].compose(doLogin, (0, _util.debugCurried)('jira', 'Current authentication failed. Attempting login')));
      } else {
        loginPromise = doLogin();
      }

      var parseIssue = _ramda2['default'].compose(_util.wrapInPromise, _ramda2['default'].converge(_ramda2['default'].set(_ramda2['default'].lensProp('url')), [_ramda2['default'].compose(_ramda2['default'].concat(issueRoot), _ramda2['default'].prop('key')), _ramda2['default'].identity]), _ramda2['default'].prop('body'));

      var addCommentToIssue = _ramda2['default'].curry(function (issue, comment) {
        return post('/rest/api/2/issue/' + issue.key + '/comment', { body: { body: comment } });
      });

      return loginPromise.then(function (user) {
        return {
          name: 'jira',
          issueRoot: issueRoot,
          getCurrentUser: _ramda2['default'].always((0, _util.wrapInPromise)(user)),
          getIssue: _ramda2['default'].composeP(parseIssue, _ramda2['default'].compose((0, _error.catchPromiseAndThrow)('jira', _error.errors.jira.issueNotFound), get, _ramda2['default'].concat(_ramda2['default'].__, '?expand=transitions'), _ramda2['default'].concat('/rest/api/2/issue/')), (0, _util.debugCurriedP)('jira', 'Getting issue from jira')),
          setIssueStatus: _ramda2['default'].curryN(2, function (_ref7, issue) {
            var issueStatus = _ref7.status,
                comment = _ref7.comment;
            return _ramda2['default'].composeP(_ramda2['default'].always(issue),
            // Jira api for transition is not adding the comment so we need an extra api call
            _ramda2['default'].partial(_ramda2['default'].unless(_ramda2['default'].isNil, addCommentToIssue(issue)), [comment]), post('/rest/api/2/issue/' + issue.key + '/transitions'), (0, _util.debugCurriedP)('jira', 'Updating issue status to ' + issueStatus))({
              body: {
                transition: _ramda2['default'].compose(_ramda2['default'].pick(['id']), _ramda2['default'].find(_ramda2['default'].compose(_ramda2['default'].equals(issueStatus), Number, _ramda2['default'].path(['to', 'id']))), _ramda2['default'].prop('transitions'))(issue),
                fields: {}
              }
            });
          }),
          canWorkOnIssue: (0, _issue2['default'])(status).canWorkOnIssue,
          status: status
        };
      });
    });
  };
};