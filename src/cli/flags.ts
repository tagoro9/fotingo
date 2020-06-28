import { flags } from '@oclif/command';

export const branch = flags.string({
  // TODO Add completion
  completion: undefined,
  description: 'Name of the base branch of the pull request',
  char: 'b',
  name: 'branch',
});

export const yes = flags.boolean({
  char: 'y',
  description: 'Do not prompt for any input but accept all the defaults',
  name: 'yes',
  default: false,
});
