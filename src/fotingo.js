#!/usr/bin/env node

import program from 'commander';
import app from '../package.json';

program
  .version(app.version)
  .command('start [issue-id]', 'start working in a new issue')
  .command('review', 'submit current issue for review')
  .command('release [release-name]', 'create a release with your changes')
  .parse(process.argv);
