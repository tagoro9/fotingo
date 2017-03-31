import readConfigFile from './read-config-file';
import writeConfigFile from './write-config-file';
import R from 'ramda';

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

const data = {
  isGithubLoggedIn: () => R.not(R.isNil(data.get(['github', 'token']))),
  isJiraLoggedIn: () =>
    R.not(R.or(R.isNil(data.get(['jira', 'user', 'password'])), R.isNil(data.get(['jira', 'user', 'login'])))),
  update: R.curryN(2, (path, value) => {
    config = R.set(R.lensPath(path), value, config);
    writeConfigFile(config.local ? R.set(R.lensPath(path), value, {}) : config, config.local);
    return value;
  }),
  get: path => R.view(R.lensPath(path), config),
};

export default data;
