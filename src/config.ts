/**
 * Configuration management
 */

import { cosmiconfigSync as cosmiconfig } from 'cosmiconfig';
import { writeFileSync } from 'fs';
import * as path from 'path';
import * as R from 'ramda';

import { GitConfig, GithubConfig } from './git/Config';
import { JiraConfig } from './issue-tracker/Config';
import { ReleaseConfig } from './types';

export interface Config {
  git: GitConfig;
  github: GithubConfig;
  jira: JiraConfig;
  release: ReleaseConfig;
}

export const requiredConfigs = [
  { path: ['jira', 'user', 'login'], request: "What's your Jira user?" },
  { path: ['jira', 'user', 'token'], request: "What's your Jira token?" },
  { path: ['jira', 'root'], request: "What's the Jira root?" },
  { path: ['github', 'authToken'], request: "What's your Github token?" },
];

const configSearch = cosmiconfig('fotingo');

/**
 * Read the configuration file in the specified folder. Go up until the user home
 * directory
 */
const readConfig: (path?: string) => string = R.compose(
  R.ifElse(R.either(R.isNil, R.propEq('isEmpty', true)), R.always({}), R.prop('config')),
  (p?: string) => configSearch.search(p),
);

/**
 * Read the fotingo configuration file. Find it up from the execution directory
 * and merge it with the file in the home directory
 */
export const read: () => Config = () =>
  R.converge(R.mergeWith(R.ifElse(R.is(Object), R.flip(R.merge), R.nthArg(0))), [
    readConfig,
    R.partial(readConfig, [process.env.HOME]),
  ])(undefined) as Config;

/**
 * Write some partial configuration into the closest found config file
 * @param config Partial configuration
 */
export const write: (data: Partial<Config>) => Partial<Config> = data => {
  const search = configSearch.search() || { filepath: undefined, config: {} };
  const mergedConfigs = R.mergeDeepLeft(data, search.config);
  writeFileSync(
    // TODO Use homedir() instead of env variable
    search.filepath || path.join(process.env.HOME as string, '.fotingorc'),
    JSON.stringify(mergedConfigs, null, 2),
    'utf-8',
  );
  return data;
};
