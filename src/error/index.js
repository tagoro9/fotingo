import R from 'ramda';
import errors from './errors';
import { debugCurried, error } from '../util';
import reporter from '../reporter';

export class ControlledError extends Error {
  constructor(message, parameters = {}) {
    super(R.compose(
      R.reduce((msg, [k, v]) => R.replace(`\${${k}}`, v, msg), message),
      R.toPairs
    )(parameters));
  }
}

export const throwControlledError = (message, parameters) => () => {
  throw new ControlledError(message, parameters);
};

// Error -> Boolean
const isKnownError = R.either(R.is(ControlledError), R.propEq('message', 'canceled'));
const exit = code => () => process.exit(code);
const userIsExiting = R.compose(R.equals('canceled'), R.prop('message'));
const handleErrorAndExit = R.compose(exit(0), error, R.prop('message'));
const handleUnknownError = R.compose(exit(1), error);
const sayBye = () => reporter.log('Hasta la vista baby!', 'wave');
export const handleError = R.ifElse(
  isKnownError,
  R.ifElse(userIsExiting, sayBye, handleErrorAndExit),
  handleUnknownError
);
// String -> Promise -> Promise
export const catchPromiseAndThrow = (module, e) => p => p.catch(err => {
  if (R.is(Function, e)) {
    throwControlledError(e(err))(err);
  } else {
    R.compose(throwControlledError(e), debugCurried(module, err))(err);
  }
});
export { errors };
