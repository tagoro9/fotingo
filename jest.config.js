module.exports = {
  roots: ['<rootDir>/src', '<rootDir>/test'],
  coverageDirectory: './coverage/',
  coverageReporters: ['html', 'lcov'],
  setupFilesAfterEnv: ['<rootDir>test/setupTests.ts'],
  preset: 'ts-jest',
  testRegex: '(/test/.*(test|spec))\\.(tsx?)$',
  moduleNameMapper: {
    'src/(.*)': '<rootDir>/src/$1',
    'test/(.*)': '<rootDir>/test/$1',
  },
};
