import path from 'path';
import app from '../../package.json';

export const CONFIG_FILE_NAME = `.${app.name}`;

export const globalConfigFilePath = path.resolve(
  process.env.HOME || process.env.USERPROFILE,
  CONFIG_FILE_NAME,
);
export const localConfigFilePath = path.resolve(process.cwd(), CONFIG_FILE_NAME);
