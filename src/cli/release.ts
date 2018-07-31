/**
 * Release command
 */
import { Argv } from 'yargs';

export const command: string = 'release <release-name>';
export const describe: string = 'Create a release with your changes';
export function builder(yargs: Argv): Argv {
  return yargs
    .positional('release-name', {
      describe: 'Name of the release to be created',
      type: 'string',
    })
    .option('issue', {
      alias: 'i',
      describe: 'Specify more issues to include in the release',
      type: 'array',
    })
    .option('yes', {
      alias: 'y',
      describe: 'Do not prompt for any input but accept all the defaults',
      type: 'boolean',
    });
}
