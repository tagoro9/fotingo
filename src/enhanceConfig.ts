import { ifElse, isNil, mergeDeepLeft, mergeDeepWith, nthArg } from 'ramda';
import { Config, IssueStatus } from 'src/types';

import { Git } from './git/Git';
import { GitErrorType } from './git/GitError';
import { getFileContent } from './io/file';

interface DefaultConfig {
  git: {
    baseBranch: string;
    branchTemplate: string;
    remote: string;
  };
  github: {
    baseBranch: string;
    pullRequestTemplate: string;
  };
  jira: {
    status: Record<IssueStatus, RegExp>;
  };
  release: {
    template: string;
  };
}

const defaultConfig: DefaultConfig = {
  git: {
    baseBranch: 'master',
    branchTemplate: '{issue.shortName}/{issue.key}_{issue.sanitizedSummary}',
    remote: 'origin',
  },
  github: {
    baseBranch: 'master',
    pullRequestTemplate:
      '{summary}\n\n**Description**\n\n{description}\n\n{fixedIssues}\n\n**Changes**\n\n{changes}\n\n{fotingo.banner}',
  },
  jira: {
    status: {
      [IssueStatus.BACKLOG]: /backlog/i,
      [IssueStatus.IN_PROGRESS]: /in progress/i,
      [IssueStatus.IN_REVIEW]: /review/i,
      [IssueStatus.DONE]: /done/i,
      [IssueStatus.SELECTED_FOR_DEVELOPMENT]: /(todo)|(to do)|(selected for development)/i,
    },
  },
  release: {
    template:
      '{version}\n\n{fixedIssuesByCategory}\n\nSee [Jira release]({jira.release})\n\n{fotingo.banner}',
  },
};

/**
 * Enhance the current configuration with overrides from the CLI arguments
 * @param config Current config
 * @param data Program data (yargs)
 */
export function enhanceConfigWithRuntimeArguments(
  config: Config,
  data: { branch?: string },
): Config {
  return mergeDeepLeft(
    data.branch !== undefined
      ? {
          git: {
            baseBranch: data.branch,
          },
          github: {
            baseBranch: data.branch,
          },
        }
      : {},
    config,
  ) as Config;
}

/**
 * Enhance the current configuration with some defaults and information that can be derived from
 * the running environment
 * @param config Current config
 */
export async function enhanceConfig(config: Config): Promise<Config> {
  const configWithDefaults = mergeDeepWith(
    ifElse(isNil, nthArg(1), nthArg(0)),
    config,
    defaultConfig,
  ) as Config;
  try {
    // TODO I don't like this instantiation of Git here
    const git = new Git(configWithDefaults.git);
    const rootDirectory = await git.getRootDir();
    const prTemplate = await getFileContent('PULL_REQUEST_TEMPLATE.md', rootDirectory, [
      '.',
      '.github',
    ]);
    return git.getRemote(configWithDefaults.git.remote).then(
      (remote) =>
        mergeDeepWith(
          ifElse(isNil, nthArg(1), nthArg(0)),
          {
            github: {
              pullRequestTemplate: prTemplate,
              owner: remote.owner,
              repo: remote.name,
            },
          },
          configWithDefaults,
        ) as Config,
    );
  } catch (error) {
    if (error.code && error.code === GitErrorType.NOT_A_GIT_REPO) {
      // Ignore the error, as it means we are running fotingo outside a repo
      return configWithDefaults;
    }
    throw error;
  }
}
