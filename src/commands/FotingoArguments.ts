/**
 * FotingoArguments. Extension of yargs Argument so we
 * can access the configuration in a typed manner
 */

import { Arguments } from 'yargs';

import { Config } from 'src/config';

export interface FotingoArguments extends Arguments {
  // User configuration
  config: Config;
}
