/**
 * FotingoArguments. Extension of yargs Argument so we
 * can access the configuration in a typed manner
 */

import { Config } from 'src/config';
import { Arguments } from 'yargs';

export interface FotingoArguments extends Arguments {
  // User configuration
  config: Config;
}
