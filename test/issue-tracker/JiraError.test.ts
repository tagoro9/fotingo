import { JiraErrorImpl } from 'src/issue-tracker/jira/JiraError';
import { describe, expect, it } from 'vitest';

describe('JiraError', () => {
  it('should store the message and the code', () => {
    const error = new JiraErrorImpl('Test message', 'ERR_CODE');
    expect(error.message).toMatchSnapshot();
    expect(error.code).toMatchSnapshot();
  });
});
