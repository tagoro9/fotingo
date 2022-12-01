import { faker } from '@faker-js/faker';
import { beforeEach } from '@jest/globals';

beforeEach(() => {
  faker.seed(2_340_928_340_293_840);
});
