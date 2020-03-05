/**
 * Start command
 */

import { Argv } from 'yargs';

export const command = 'start [issue-id|issue-title]';
export const describe = 'Start working in a new issue';
export function builder(yargs: Argv): Argv {
  return yargs
    .positional('issue-id', {
      describe: 'Id / description of the issue to start working with',
      type: 'string',
    })
    .option('branch', {
      alias: 'b',
      describe: 'Name of the branch to use',
      type: 'string',
    })
    .option('no-branch-issue', {
      alias: 'n',
      default: false,
      describe: 'Do not create a branch with the issue name',
      type: 'boolean',
    })
    .option('create', {
      alias: 'c',
      describe: 'Create a new issue instead of searching for it',
      implies: ['project', 'type'],
      type: 'boolean',
    })
    .option('project', {
      alias: 'p',
      describe: 'Name of the project where to create the issue',
      requiresArg: true,
      type: 'string',
    })
    .option('type', {
      alias: 't',
      describe: 'Type of issue to be created',
      requiresArg: true,
      type: 'string',
    })
    .option('description', {
      alias: 'd',
      describe: 'Description of the issue to be created',
      requiresArg: true,
      type: 'string',
    })
    .option('labels', {
      alias: 'l',
      array: true,
      describe: 'Labels to add to the issue',
    });
}
