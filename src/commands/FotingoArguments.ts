/**
 * FotingoArguments. Extension of yargs Argument so we
 * can access the configuration in a typed manner
 */

import { Config } from 'src/config';
import { Arguments } from 'yargs';

export interface FotingoArguments extends Arguments {
  // Optional branch that most of commands have as option
  branch?: string;
  // User configuration
  config: Config;
}
