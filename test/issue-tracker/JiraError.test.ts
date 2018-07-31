import 'jest';
import { JiraErrorImpl } from 'src/issue-tracker/JiraError';

describe('JiraError', () => {
  it('should store the message and the code', () => {
    const error = new JiraErrorImpl('Test message', 'ERR_CODE');
    expect(error.message).toMatchSnapshot();
    expect(error.code).toMatchSnapshot();
  });
});
