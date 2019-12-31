/**
 * Review command
 */
import { Argv } from 'yargs';

export const command = 'review';
export const describe = 'Submit current issue for review';
export function builder(yargs: Argv): Argv {
  return yargs
    .option('branch', {
      alias: 'b',
      describe: 'Name of the base branch of the pull request',
      type: 'string',
    })
    .option('label', {
      alias: 'l',
      describe: 'Label to add to the PR',
      requiresArg: true,
      type: 'array',
    })
    .option('reviewer', {
      alias: 'r',
      describe: 'Request some people to review your PR',
      requiresArg: true,
      type: 'array',
    })
    .option('simple', {
      alias: 's',
      describe: 'Do not use any issue tracker',
      type: 'boolean',
    })
    .option('yes', {
      alias: 'y',
      describe: 'Do not prompt for any input but accept all the defaults',
      type: 'boolean',
    });
}
