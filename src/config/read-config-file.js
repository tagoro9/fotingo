import fs from 'fs';
import R from 'ramda';
import { globalConfigFilePath, localConfigFilePath } from './config-file-path';
import { ControlledError, errors, handleError } from '../error';

const createEmptyConfigFile = R.curryN(3, fs.writeFileSync)(R.__, R.__, 'utf8');
const readConfigFile = R.curryN(2, R.compose(JSON.parse, fs.readFileSync))(R.__, 'utf8');

const getGlobalConfig = R.tryCatch(
  R.compose(readConfigFile, R.always(globalConfigFilePath)),
  R.ifElse(
    R.propEq('code', 'ENOENT'),
    R.converge(R.identity, [
      R.nthArg(1),
      R.compose(
        createEmptyConfigFile(globalConfigFilePath),
        defaults => JSON.stringify(defaults, null, 2),
        R.nthArg(1),
      ),
    ]),
    () => handleError(new ControlledError(errors.config.malformedFile)),
  ),
);

export const getLocalConfig = R.tryCatch(
  R.compose(R.set(R.lensProp('local'), true), readConfigFile, R.always(localConfigFilePath)),
  R.ifElse(R.propEq('code', 'ENOENT'), R.always({}), () =>
    handleError(new ControlledError(errors.config.malformedFile)),
  ),
);

export default R.converge(R.mergeWith(R.ifElse(R.is(Object), R.merge, R.nthArg(1))), [
  getGlobalConfig,
  getLocalConfig,
]);
