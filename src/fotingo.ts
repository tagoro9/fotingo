#!/usr/bin/env node
/**
 * Fotingo entry point. Read configuration and CLI input.
 * Parse and run the command
 */

// tslint:disable-next-line no-var-requires no-require-imports
require('module-alias/register');

import { compose, prop } from 'ramda';
import { commandDir } from 'yargs';

import { run } from 'src/commands/run';
import { Config, read } from 'src/config';
import { enhanceConfig } from 'src/enhanceConfig';

enhanceConfig(read()).then((config: Config) => {
  const program = commandDir('./cli')
    .demandCommand()
    .config({ config })
    .completion()
    .help();

  compose(
    run,
    prop('argv'),
  )(program);
});
