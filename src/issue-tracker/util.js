import R from 'ramda';
import { throwControlledError, errors } from '../error';

const isPresent = R.complement(R.either(R.isNil, R.isEmpty));
const issueRegex = new RegExp('^([a-zA-Z]+)\\-(\\d+)$');
const isValidName = R.test(issueRegex);
const isInvalid = R.complement(R.both(isPresent, isValidName));
// string -> boolean
// eslint-disable-next-line import/prefer-default-export
export const validate = R.when(isInvalid, throwControlledError(errors.jira.issueIdNotValid));
