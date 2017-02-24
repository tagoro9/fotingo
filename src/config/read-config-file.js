import fs from 'fs';
import R from 'ramda';
import configFilePath from './config-file-path';
import { ControlledError, errors, handleError } from '../error';

const createEmptyConfigFile = R.curryN(3, fs.writeFileSync)(R.__, R.__, 'utf8');
const readConfigFile = R.curryN(2, R.compose(JSON.parse, fs.readFileSync))(R.__, 'utf8');

// export default R.compose(JSON.parse, R.tryCatch(readUtf8File, curriedWriteFileSync, {}))(filePath);


export default (defaults) => {
  try {
    return readConfigFile(configFilePath);
  } catch (e) {
    if (e.code && e.code === 'ENOENT') {
      createEmptyConfigFile(configFilePath, JSON.stringify(defaults, null, 2));
      return defaults;
    }
    return handleError(new ControlledError(errors.config.malformedFile));
  }
};
