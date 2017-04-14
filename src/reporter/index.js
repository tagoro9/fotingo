import format, { supportsColor } from 'chalk';
import readline from 'readline';
import emojis from 'node-emoji';
import R from 'ramda';
import read from 'read';

const stdout = process.stdout;
const startTime = Date.now();
const isTTY = stdout.isTTY;

const clearLine = () => {
  if (!supportsColor) {
    return;
  }
  readline.clearLine(stdout, 0);
  readline.cursorTo(stdout, 0);
};

const prependEmoji = (msg, emoji) => {
  if (emoji && isTTY) {
    return `${emoji}  ${msg}`;
  }
  return msg;
};

const log = (msg, emojiStr) => {
  clearLine();
  stdout.write(`${emojiStr ? prependEmoji(msg, emojis.get(emojiStr)) : msg}\n`);
};

const step = R.curryN(4, (total, current, msg, emojiStr) => {
  const actualMsg = prependEmoji(msg, emojis.get(emojiStr));
  log(`${format.grey(`[${current}/${total}]`)} ${actualMsg}...`);
});

const stepCurried = R.curryN(5, (total, current, msg, emojiStr, args) => {
  step(total, current, R.ifElse(R.is(Function), m => m(args), R.identity())(msg), emojiStr);
  return args;
});

const stepCurriedP = R.curryN(5, (total, current, msg, emojiStr, args) => {
  stepCurried(total, current, msg, emojiStr, args);
  return Promise.resolve(args);
});

export default {
  stepFactory(totalSteps) {
    return {
      step: step(totalSteps),
      stepCurried: stepCurried(totalSteps),
      stepCurriedP: stepCurriedP(totalSteps),
    };
  },
  log,
  info: msg => log(`${format.grey('info')} ${msg}`),
  error(msg) {
    log(`${format.red('error')} ${prependEmoji(msg, emojis.get('boom'))}`);
  },
  question({ question, password = false }) {
    return new Promise((resolve, reject) =>
      read({ silent: password, prompt: `${format.grey('question')} ${question}:` }, (err, text) => {
        if (err) {
          return reject(err);
        }
        return resolve(text);
      }),
    );
  },
  footer(artifact) {
    const totalTime = ((Date.now() - startTime) / 1000).toFixed(2);
    const msg = `Done in ${totalTime}s ${artifact ? `=> ${artifact}` : '.'}`;
    log(prependEmoji(msg, emojis.get('sparkles')));
  },
};
