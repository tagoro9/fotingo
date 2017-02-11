import fs from 'fs';
import configFilePath from './config-file-path';

export default (data) =>
  fs.writeFileSync(configFilePath, JSON.stringify(data, null, 2), { encoding: 'utf8', flag: 'w' });
