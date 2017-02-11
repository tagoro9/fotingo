'use strict';

Object.defineProperty(exports, "__esModule", {
  value: true
});

var _chalk = require('chalk');

var _chalk2 = _interopRequireDefault(_chalk);

var _readline = require('readline');

var _readline2 = _interopRequireDefault(_readline);

var _nodeEmoji = require('node-emoji');

var _nodeEmoji2 = _interopRequireDefault(_nodeEmoji);

var _ramda = require('ramda');

var _ramda2 = _interopRequireDefault(_ramda);

var _read = require('read');

var _read2 = _interopRequireDefault(_read);

function _interopRequireDefault(obj) { return obj && obj.__esModule ? obj : { 'default': obj }; }

var stdout = process.stdout;
var startTime = Date.now();
var isTTY = stdout.isTTY;

var clearLine = function () {
  function clearLine() {
    if (!_chalk.supportsColor) {
      return;
    }
    _readline2['default'].clearLine(stdout, 0);
    _readline2['default'].cursorTo(stdout, 0);
  }

  return clearLine;
}();

var prependEmoji = function () {
  function prependEmoji(msg, emoji) {
    if (emoji && isTTY) {
      return emoji + '  ' + msg;
    }
    return msg;
  }

  return prependEmoji;
}();

var log = function () {
  function log(msg, emojiStr) {
    clearLine();
    stdout.write((emojiStr ? prependEmoji(msg, _nodeEmoji2['default'].get(emojiStr)) : msg) + '\n');
  }

  return log;
}();

var step = _ramda2['default'].curryN(4, function (total, current, msg, emojiStr) {
  var actualMsg = prependEmoji(msg, _nodeEmoji2['default'].get(emojiStr));
  log(_chalk2['default'].grey('[' + current + '/' + total + ']') + ' ' + actualMsg + '...');
});

var stepCurried = _ramda2['default'].curryN(5, function (total, current, msg, emojiStr, args) {
  step(total, current, msg, emojiStr);
  return args;
});

exports['default'] = {
  stepFactory: function () {
    function stepFactory(totalSteps) {
      return { step: step(totalSteps), stepCurried: stepCurried(totalSteps) };
    }

    return stepFactory;
  }(),

  log: log,
  error: function () {
    function error(msg) {
      log(_chalk2['default'].red('error') + ' ' + prependEmoji(msg, _nodeEmoji2['default'].get('boom')));
    }

    return error;
  }(),
  question: function () {
    function question(_ref) {
      var _question = _ref.question,
          _ref$password = _ref.password,
          password = _ref$password === undefined ? false : _ref$password;

      return new Promise(function (resolve, reject) {
        return (0, _read2['default'])({ silent: password, prompt: _chalk2['default'].grey('question') + ' ' + _question + ':' }, function (err, text) {
          if (err) {
            return reject(err);
          }
          return resolve(text);
        });
      });
    }

    return question;
  }(),
  footer: function () {
    function footer() {
      var totalTime = ((Date.now() - startTime) / 1000).toFixed(2);
      var msg = 'Done in ' + totalTime + 's.';
      log(prependEmoji(msg, _nodeEmoji2['default'].get('sparkles')));
    }

    return footer;
  }()
};