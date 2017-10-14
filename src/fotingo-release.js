import program from 'commander';
import R from 'ramda';

import { handleError } from './error';
import config from './config';
import init from './init';
import reporter from './reporter';

// const getReleaseName = R.compose(R.head, R.prop('args'));

try {
  program
    .option('-n, --no-branch-issue', 'Do not pick issue from the branch name')
    .option('-s, --simple', 'Do not use any issue tracker')
    .option('-i, --issue [issue]', 'Specify more issues to include in the release', R.append, [])
    .parse(process.argv);

  const { step } = reporter.stepFactory(1);
  step(1, 'Initializing services', 'rocket');
  init(config, program)
    .then(R.partial(reporter.footer, [null]))
    .catch(handleError);
} catch (e) {
  handleError(e);
  program.help();
}
