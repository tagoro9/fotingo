import fs from 'fs';
import { globalConfigFilePath, localConfigFilePath } from './config-file-path';

export default (data, local = false) =>
  fs.writeFileSync(
    local ? localConfigFilePath : globalConfigFilePath,
    JSON.stringify(data, null, 2),
    {
      encoding: 'utf8',
      flag: 'w',
    },
  );
