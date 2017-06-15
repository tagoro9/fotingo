import R from 'ramda';
import readConfigFile, { getLocalConfig } from './read-config-file';
import writeConfigFile from './write-config-file';

let config = readConfigFile({
  git: {
    remote: 'origin',
    branch: 'master',
  },
  github: {
    base: 'master',
  },
  jira: {
    status: {},
    user: {},
  },
});

let inMemoryConfig = {};
const set = (path, value, obj) => R.set(R.lensPath(path), value, obj);
const get = R.curryN(2, (obj, path) => R.view(R.lensPath(path), obj));
function recursiveMerge(a, b = {}) {
  return R.mergeWith(R.ifElse(R.is(Object), recursiveMerge, R.flip(R.or)))(a, b);
}

const data = {
  isGithubLoggedIn: () => R.not(R.isNil(data.get(['github', 'token']))),
  isJiraLoggedIn: () =>
    R.not(
      R.or(
        R.isNil(data.get(['jira', 'user', 'password'])),
        R.isNil(data.get(['jira', 'user', 'login'])),
      ),
    ),
  update: R.curryN(2, (path, value, inMemory) => {
    if (inMemory) {
      inMemoryConfig = set(path, value, inMemoryConfig);
    } else {
      config = set(path, value, config);
      writeConfigFile(
        config.local ? R.set(R.lensPath(path), value, R.omit(['local'], getLocalConfig())) : config,
        config.local,
      );
    }
    return value;
  }),
  get: R.converge(R.ifElse(R.is(Object), recursiveMerge, (file, memory) => memory || file), [
    path => get(config, path),
    path => get(inMemoryConfig, path),
  ]),
};

export default data;
