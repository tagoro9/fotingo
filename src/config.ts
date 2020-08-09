/**
 * Configuration management
 */

import { cosmiconfigSync as cosmiconfig } from 'cosmiconfig';
import { writeFileSync } from 'fs';
import * as path from 'path';
import * as R from 'ramda';

import { Config } from './types';

export const requiredConfigs = [
  { path: ['jira', 'user', 'login'], request: "What's your Jira user?" },
  { path: ['jira', 'user', 'token'], request: "What's your Jira token?" },
  { path: ['jira', 'root'], request: "What's the Jira root?" },
  { path: ['github', 'authToken'], request: "What's your Github token?" },
];

// Object with the logic to deserialize config values such a regexes
const configDeserializer = [
  {
    path: ['jira', 'status'],
    deserialize: R.ifElse(
      R.isNil,
      R.identity,
      R.mapObjIndexed((statusRegex: string) => new RegExp(statusRegex, 'i')),
    ),
  },
];

/**
 * Read the configuration file in the specified folder. Go up until the user home
 * directory
 */
const readConfigFile: (path?: string) => string = R.compose(
  R.ifElse(R.either(R.isNil, R.propEq('isEmpty', true)), R.always({}), R.prop('config')),
  (p?: string) => cosmiconfig('fotingo').search(p),
);

/**
 * Read the fotingo configuration file. Find it up from the execution directory
 * and merge it with the file in the home directory
 */
export const readConfig: () => Config = R.compose(
  (config) =>
    R.reduce(
      (newConfig, deserializer) => {
        return R.ifElse(
          R.pathSatisfies(R.isNil, deserializer.path),
          R.identity,
          R.set(
            R.lensPath(deserializer.path),
            deserializer.deserialize(R.path(deserializer.path, newConfig)),
          ),
        )(newConfig) as Config;
      },
      config,
      configDeserializer,
    ),
  () =>
    R.converge(R.mergeWith(R.ifElse(R.is(Object), R.flip(R.merge), R.nthArg(0))), [
      readConfigFile,
      R.partial(readConfigFile, [process.env.HOME]),
    ])(undefined),
);

/**
 * Write some partial configuration into the closest found config file
 * @param config Partial configuration
 */
export const writeConfig: (data: Partial<Config>) => Partial<Config> = (data) => {
  const search = cosmiconfig('fotingo').search() || { filepath: undefined, config: {} };
  const mergedConfigs = R.mergeDeepLeft(data, search.config);
  writeFileSync(
    // TODO Use homedir() instead of env variable
    search.filepath || path.join(process.env.HOME as string, '.fotingorc'),
    JSON.stringify(mergedConfigs, undefined, 2),
    'utf-8',
  );
  return data;
};
