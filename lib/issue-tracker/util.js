'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});
exports.validate = undefined;

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _error = require('../error');

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var isPresent = _ramda2['default'].complement(_ramda2['default'].either(_ramda2['default'].isNil, _ramda2['default'].isEmpty));
var issueRegex = new RegExp('^([a-zA-Z]+)\\-(\\d+)$');
var isValidName = _ramda2['default'].test(issueRegex);
var isInvalid = _ramda2['default'].complement(_ramda2['default'].both(isPresent, isValidName));
// string -> boolean
var validate = exports.validate = _ramda2['default'].when(isInvalid, (0, _error.throwControlledError)(_error.errors.jira.issueIdNotValid));