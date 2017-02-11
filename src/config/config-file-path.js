import path from 'path';
import app from '../../package.json';
const CONFIG_FILE_NAME = `.${app.name}`;

export default path.resolve(process.env.HOME || process.env.USERPROFILE, CONFIG_FILE_NAME);
