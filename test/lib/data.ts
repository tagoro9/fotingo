import * as faker from 'faker';
import { GitConfig } from 'src/git/Config';
import { JiraConfig } from 'src/issue-tracker/jira/Config';
import { JiraIssue } from 'src/issue-tracker/jira/types';
import { HttpResponse } from 'src/network/HttpClient';
import { Issue, IssueType, User } from 'src/types';

/**
 * Data factory used to generate mock data for the tests
 */
export const data = {
  createJiraIssue(): JiraIssue {
    const summary = faker.name.jobDescriptor();
    const sanitizedSummary = faker.helpers.slugify(summary);
    const issueType = faker.random.arrayElement(Object.values(IssueType));
    return {
      fields: {
        description: faker.lorem.paragraph(),
        issuetype: {
          name: issueType,
        },
        project: {
          id: faker.lorem.word(),
        },
        summary,
      },
      id: faker.random.number(5000),
      key: `FOTINGO-${faker.random.number(5000)}`,
      renderedFields: {
        description: undefined,
      },
      sanitizedSummary,
      transitions: [],
      url: faker.internet.url(),
    };
  },
  createIssue(): Issue {
    const summary = faker.name.jobDescriptor();
    const sanitizedSummary = faker.helpers.slugify(summary);
    const issueType = faker.random.arrayElement(Object.values(IssueType));
    return {
      description: faker.lorem.paragraph(),
      id: faker.random.number(5000),
      key: `FOTINGO-${faker.random.number(5000)}`,
      project: faker.lorem.word(),
      sanitizedSummary,
      summary,
      type: issueType,
      url: faker.internet.url(),
    };
  },
  createGitConfig(): GitConfig {
    return {
      baseBranch: 'master',
      branchTemplate: '{issue.key}',
      remote: 'origin',
    };
  },
  createJiraConfig(): JiraConfig {
    return {
      root: faker.internet.url(),
      user: {
        login: faker.internet.email(),
        token: faker.internet.password(),
      },
    };
  },
  createJiraUser(): User {
    return {
      accountId: faker.internet.userName(),
      groups: {
        items: {
          name: 'asda',
        },
      },
    };
  },
  createHttpResponse<T>(body: T): HttpResponse<T> {
    return {
      body,
      response: {},
    };
  },
};
