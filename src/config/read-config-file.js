import fs from 'fs';
import R from 'ramda';
import configFilePath from './config-file-path';

const createEmptyConfigFile = R.curryN(3, fs.writeFileSync)(R.__, R.__, 'utf8');
const readConfigFile = R.curryN(2, R.compose(JSON.parse, fs.readFileSync))(R.__, 'utf8');

// export default R.compose(JSON.parse, R.tryCatch(readUtf8File, curriedWriteFileSync, {}))(filePath);


export default (defaults) => {
  try {
    return readConfigFile(configFilePath);
  } catch (e) {
    createEmptyConfigFile(configFilePath, JSON.stringify(defaults));
    return defaults;
  }
};
