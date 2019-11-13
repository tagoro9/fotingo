export interface JiraConfig {
  user: {
    login: string;
    token: string;
  };
  releaseTemplate: string;
  root: string;
}
